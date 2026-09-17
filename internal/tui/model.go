package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/eberureon/daylog/internal/app"
	"github.com/eberureon/daylog/internal/domain"
	"github.com/google/uuid"
)

type Mode int

const (
	TasksMode Mode = iota
	JournalMode
	TodayMode
	TrashMode
)

type item struct {
	task        *domain.Task
	journal     *domain.JournalEntry
	dateHeading string
}

func (i item) FilterValue() string {
	if i.task != nil {
		return i.task.Description
	}
	return i.journal.Description
}

func (i item) Title() string {
	if i.task == nil {
		title := "• " + i.journal.Description
		if i.dateHeading != "" {
			return i.dateHeading + "\n" + title
		}
		return title
	}
	if i.dateHeading != "" {
		return i.dateHeading + "\n" + i.taskTitle()
	}
	return i.taskTitle()
}

func (i item) taskTitle() string {
	prefix := "○"
	if i.task.Status == domain.TaskDone {
		prefix = "✓"
	}
	if i.task.Status == domain.TaskArchived {
		prefix = "→"
	}
	if i.task.Important {
		prefix = "!" + prefix
	}
	return prefix + " " + i.task.Description
}

func (i item) Description() string {
	if i.task != nil {
		parts := []string{string(i.task.Status)}
		if i.task.Project != "" {
			parts = append(parts, "project: "+i.task.Project)
		}
		if i.task.DueDate != nil {
			parts = append(parts, "due: "+i.task.DueDate.Format("2006-01-02"))
		}
		return strings.Join(parts, "  ·  ")
	}
	parts := []string{"journal"}
	if i.journal.Project != "" {
		parts = append(parts, "project: "+i.journal.Project)
	}
	return strings.Join(parts, "  ·  ")
}

type loadedMsg struct {
	tasks    []domain.Task
	journal  []domain.JournalEntry
	reviewed bool
	err      error
}

type changedMsg struct{ err error }

type formKind int

const (
	taskForm formKind = iota
	journalForm
)

type formModel struct {
	kind        formKind
	fields      []textinput.Model
	focus       int
	error       string
	editTask    *domain.Task
	editJournal *domain.JournalEntry
}

type Model struct {
	app       *app.App
	mode      Mode
	list      list.Model
	tasks     []domain.Task
	journal   []domain.JournalEntry
	width     int
	height    int
	help      bool
	status    string
	loadError error
	form      *formModel
	confirm   string
	reviewed  bool
}

var (
	activeStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	taskStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	journalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func New(a *app.App) Model {
	delegate := list.NewDefaultDelegate()
	// Date headings are attached to the first entry in each group, so they
	// are labels rather than selectable list rows.
	delegate.SetHeight(3)
	// Keep a compact gap between entries and between date groups. The first
	// entry's title contains its date heading, so there is no gap between a
	// heading and that entry.
	delegate.SetSpacing(1)
	l := list.New(nil, delegate, 0, 0)
	l.Title = "daylog"
	l.SetShowStatusBar(true)
	return Model{app: a, list: l}
}

func (m Model) Init() tea.Cmd { return m.load }

func (m Model) load() tea.Msg {
	includeDeleted := m.mode == TrashMode
	tasks, err := m.app.ListTasks(context.Background(), includeDeleted)
	if err != nil {
		return loadedMsg{err: err}
	}
	journal, err := m.app.ListJournalEntries(context.Background(), includeDeleted)
	reviewed, reviewErr := m.app.IsReviewed(context.Background(), time.Now().In(time.Local).Format("2006-01-02"))
	if err == nil {
		err = reviewErr
	}
	return loadedMsg{tasks: tasks, journal: journal, reviewed: reviewed, err: err}
}

func (m Model) mutate(action func() error) tea.Cmd {
	return func() tea.Msg { return changedMsg{err: action()} }
}

func newForm(kind formKind) *formModel {
	labels := []string{"Description", "Project", "Tags"}
	if kind == taskForm {
		labels = append(labels, "Due date")
	} else {
		labels = append(labels, "Entry date", "Task IDs")
	}
	fields := make([]textinput.Model, len(labels))
	for i, label := range labels {
		fields[i] = textinput.New()
		fields[i].Prompt = label + ": "
		fields[i].CharLimit = 500
	}
	fields[0].Focus()
	return &formModel{kind: kind, fields: fields}
}

func formForTask(task domain.Task) *formModel {
	f := newForm(taskForm)
	f.editTask = &task
	f.fields[0].SetValue(task.Description)
	f.fields[1].SetValue(task.Project)
	f.fields[2].SetValue(strings.Join(task.Tags, ", "))
	if task.DueDate != nil {
		f.fields[3].SetValue(task.DueDate.Format("2006-01-02"))
	}
	return f
}

func formForJournal(entry domain.JournalEntry) *formModel {
	f := newForm(journalForm)
	f.editJournal = &entry
	f.fields[0].SetValue(entry.Description)
	f.fields[1].SetValue(entry.Project)
	f.fields[2].SetValue(strings.Join(entry.Tags, ", "))
	if entry.EntryDate != nil {
		f.fields[3].SetValue(entry.EntryDate.Format("2006-01-02"))
	}
	ids := make([]string, 0, len(entry.TaskIDs))
	for _, id := range entry.TaskIDs {
		ids = append(ids, id.String())
	}
	f.fields[4].SetValue(strings.Join(ids, ", "))
	return f
}

func (f *formModel) move(delta int) {
	f.fields[f.focus].Blur()
	f.focus = (f.focus + delta + len(f.fields)) % len(f.fields)
	f.fields[f.focus].Focus()
}

func (f *formModel) values() (string, string, []string, *time.Time, []uuid.UUID, error) {
	description := strings.TrimSpace(f.fields[0].Value())
	if description == "" {
		return "", "", nil, nil, nil, domain.ErrEmptyDescription
	}
	project := strings.TrimSpace(f.fields[1].Value())
	var tags []string
	for _, raw := range strings.Split(f.fields[2].Value(), ",") {
		if tag := strings.TrimSpace(raw); tag != "" {
			tags = append(tags, tag)
		}
	}
	dateIndex := 3
	var date *time.Time
	if value := strings.TrimSpace(f.fields[dateIndex].Value()); value != "" {
		parsed, err := domain.ParseDate(value, time.Now())
		if err != nil {
			return "", "", nil, nil, nil, err
		}
		date = &parsed
	}
	var taskIDs []uuid.UUID
	if f.kind == journalForm {
		for _, raw := range strings.Split(f.fields[4].Value(), ",") {
			if value := strings.TrimSpace(raw); value != "" {
				id, err := uuid.Parse(value)
				if err != nil {
					return "", "", nil, nil, nil, fmt.Errorf("invalid task ID %q", value)
				}
				taskIDs = append(taskIDs, id)
			}
		}
	}
	return description, project, tags, date, taskIDs, nil
}

func (m *Model) refreshList() {
	var items []list.Item
	switch m.mode {
	case TasksMode:
		for i := range m.tasks {
			items = append(items, item{task: &m.tasks[i]})
		}
	case JournalMode:
		items = append(items, journalItems(m.journal)...)
	case TodayMode:
		items = append(items, todayTaskItems(m.tasks)...)
		items = append(items, journalItems(m.journal)...)
	case TrashMode:
		for i := range m.tasks {
			if m.tasks[i].DeletedAt != nil {
				items = append(items, item{task: &m.tasks[i]})
			}
		}
		for i := range m.journal {
			if m.journal[i].DeletedAt != nil {
				items = append(items, item{journal: &m.journal[i]})
			}
		}
	}
	m.list.SetItems(items)
	m.list.Title = []string{"Tasks", "Journal", "Today", "Trash"}[m.mode]
}

func todayTaskItems(tasks []domain.Task) []list.Item {
	var items []list.Item
	sections := []struct {
		title string
		match func(domain.Task) bool
	}{
		{"OVERDUE", func(t domain.Task) bool { return t.Status == domain.TaskOpen && isOverdue(t) }},
		{"DUE TODAY", func(t domain.Task) bool { return t.Status == domain.TaskOpen && isDueToday(t) }},
		{"IMPORTANT", func(t domain.Task) bool {
			return t.Status == domain.TaskOpen && t.Important && !isOverdue(t) && !isDueToday(t)
		}},
		{"COMPLETED TODAY", func(t domain.Task) bool {
			return t.Status == domain.TaskDone && t.CompletedAt != nil && sameLocalDate(*t.CompletedAt, time.Now())
		}},
	}
	for _, section := range sections {
		added := false
		for i := range tasks {
			if section.match(tasks[i]) {
				if !added {
					items = append(items, item{task: &tasks[i], dateHeading: section.title})
					added = true
				} else {
					items = append(items, item{task: &tasks[i]})
				}
			}
		}
	}
	return items
}

func isDueToday(task domain.Task) bool {
	if task.DueDate == nil {
		return false
	}
	return sameLocalDate(*task.DueDate, time.Now())
}
func isOverdue(task domain.Task) bool {
	if task.DueDate == nil {
		return false
	}
	today := time.Now().In(time.Local)
	return task.DueDate.In(time.Local).Before(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local))
}
func sameLocalDate(a, b time.Time) bool {
	a, b = a.In(time.Local), b.In(time.Local)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func journalItems(entries []domain.JournalEntry) []list.Item {
	items := make([]list.Item, 0, len(entries))
	lastDate := ""
	for i := range entries {
		entryDate := entries[i].CreatedAt.In(time.Local).Format("2006-01-02")
		if entries[i].EntryDate != nil {
			entryDate = entries[i].EntryDate.In(time.Local).Format("2006-01-02")
		}
		if entryDate != lastDate {
			lastDate = entryDate
			items = append(items, item{journal: &entries[i], dateHeading: entryDate})
			continue
		}
		items = append(items, item{journal: &entries[i]})
	}
	return items
}

func isDueTodayOrOverdue(task domain.Task) bool {
	if task.DueDate == nil {
		return false
	}
	today := time.Now().In(time.Local)
	due := task.DueDate.In(time.Local)
	return !due.After(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, 1))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height-5)
	case loadedMsg:
		m.loadError = msg.err
		if msg.err == nil {
			m.tasks, m.journal, m.reviewed = msg.tasks, msg.journal, msg.reviewed
			m.refreshList()
		}
	case changedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = "Saved"
			return m, m.load
		}
	case tea.KeyMsg:
		if m.confirm != "" {
			if msg.String() == "y" || msg.String() == "enter" {
				kind, idValue, _ := strings.Cut(m.confirm, ":")
				id, _ := uuid.Parse(idValue)
				m.confirm = ""
				if kind == "journal" {
					return m, m.mutate(func() error { return m.app.DeleteJournalEntry(context.Background(), id) })
				}
				return m, m.mutate(func() error { return m.app.DeleteTask(context.Background(), id) })
			}
			if msg.String() == "n" || msg.String() == "esc" {
				m.confirm = ""
			}
			return m, nil
		}
		if m.form != nil {
			switch msg.String() {
			case "esc":
				m.form = nil
				return m, nil
			case "tab", "down":
				m.form.move(1)
				return m, nil
			case "shift+tab", "up":
				m.form.move(-1)
				return m, nil
			case "enter":
				description, project, tags, date, taskIDs, err := m.form.values()
				if err != nil {
					m.form.error = err.Error()
					return m, nil
				}
				form := m.form
				kind := form.kind
				m.form = nil
				if kind == taskForm && form.editTask != nil {
					task := *form.editTask
					task.Description, task.Project, task.Tags, task.DueDate = description, project, tags, date
					return m, m.mutate(func() error { return m.app.UpdateTask(context.Background(), task) })
				}
				if kind == journalForm && form.editJournal != nil {
					entry := *form.editJournal
					entry.Description, entry.Project, entry.Tags, entry.EntryDate, entry.TaskIDs = description, project, tags, date, taskIDs
					return m, m.mutate(func() error { return m.app.UpdateJournalEntry(context.Background(), entry) })
				}
				if kind == taskForm {
					return m, m.mutate(func() error {
						_, err := m.app.AddTask(context.Background(), description, project, tags, date, false)
						return err
					})
				}
				return m, m.mutate(func() error {
					_, err := m.app.AddJournalEntry(context.Background(), description, project, tags, date, taskIDs)
					return err
				})
			}
			var cmd tea.Cmd
			m.form.fields[m.form.focus], cmd = m.form.fields[m.form.focus].Update(msg)
			return m, cmd
		}
		// Bubble's list owns the filter input. While it is being edited, every
		// key must go to that input instead of triggering application actions.
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		if m.help {
			if msg.String() == "?" || msg.String() == "esc" {
				m.help = false
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help = true
			return m, nil
		case "1":
			m.mode = TasksMode
			m.refreshList()
			return m, nil
		case "2":
			m.mode = JournalMode
			m.refreshList()
			return m, nil
		case "3":
			m.mode = TodayMode
			m.refreshList()
			return m, nil
		case "4":
			m.mode = TrashMode
			m.refreshList()
			return m, nil
		case "r":
			return m, m.load
		case "a", "e":
			if msg.String() == "e" {
				selected, ok := m.list.SelectedItem().(item)
				if !ok {
					return m, nil
				}
				if selected.task != nil {
					m.form = formForTask(*selected.task)
				} else if selected.journal != nil {
					m.form = formForJournal(*selected.journal)
				}
				return m, nil
			}
			if m.mode == JournalMode {
				m.form = newForm(journalForm)
			} else {
				m.form = newForm(taskForm)
			}
			return m, nil
		case "x", "p", "d", "u":
			selected, ok := m.list.SelectedItem().(item)
			if !ok {
				return m, nil
			}
			if msg.String() == "d" && selected.journal != nil {
				m.confirm = "journal:" + selected.journal.ID.String()
				return m, nil
			}
			if msg.String() == "u" {
				if selected.task != nil {
					id := selected.task.ID
					return m, m.mutate(func() error { return m.app.RestoreTask(context.Background(), id) })
				}
				if selected.journal != nil {
					id := selected.journal.ID
					return m, m.mutate(func() error { return m.app.RestoreJournalEntry(context.Background(), id) })
				}
			}
			if selected.task == nil {
				return m, nil
			}
			id := selected.task.ID
			switch msg.String() {
			case "x":
				if selected.task.Status == domain.TaskDone {
					return m, m.mutate(func() error { return m.app.ReopenTask(context.Background(), id) })
				}
				return m, m.mutate(func() error { return m.app.CompleteTask(context.Background(), id) })
			case "p":
				return m, m.mutate(func() error { return m.app.ToggleImportant(context.Background(), id) })
			case "d":
				m.confirm = "task:" + id.String()
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.confirm != "" {
		label := "task"
		if strings.HasPrefix(m.confirm, "journal:") {
			label = "journal entry"
		}
		return lipgloss.NewStyle().Padding(2).Render("Delete this " + label + "?  y/n")
	}
	if m.form != nil {
		return m.formView()
	}
	if m.help {
		return m.helpView()
	}
	nav := activeStyle.Render("1 Tasks") + "  " + activeStyle.Render("2 Journal") + "  " + activeStyle.Render("3 Today")
	if m.mode == TasksMode {
		nav = taskStyle.Render("1 Tasks") + "  2 Journal  3 Today"
	}
	if m.mode == JournalMode {
		nav = "1 Tasks  " + journalStyle.Render("2 Journal") + "  3 Today"
	}
	if m.mode == TodayMode {
		nav = "1 Tasks  2 Journal  " + activeStyle.Render("3 Today")
	}
	if m.mode == TrashMode {
		nav = "1 Tasks  2 Journal  3 Today  " + activeStyle.Render("4 Trash")
	}
	if m.mode == TodayMode {
		reviewStatus := "review pending"
		if m.reviewed {
			reviewStatus = "review complete"
		}
		nav += "  " + mutedStyle.Render(reviewStatus)
	}
	header := lipgloss.NewStyle().Padding(0, 1).Render(nav)
	if m.loadError != nil {
		return header + "\n\n" + m.loadError.Error()
	}
	body := m.list.View()
	if m.width >= 100 {
		selected := m.list.SelectedItem()
		detail := mutedStyle.Render("Select an item")
		if selected != nil {
			detail = lipgloss.NewStyle().Width(m.width / 3).Padding(1).Render(selected.(item).Description())
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(m.width*2/3).Render(body), detail)
	}
	footer := mutedStyle.Render("a add  e edit  x done  p important  d delete  u restore  R review  ? help  q quit")
	if m.status != "" {
		footer += "  " + m.status
	}
	return header + "\n" + body + "\n" + footer
}

func (m Model) formView() string {
	title := "Add task"
	if m.form.kind == journalForm {
		title = "Add journal entry"
	}
	lines := []string{activeStyle.Render(title), ""}
	for _, field := range m.form.fields {
		lines = append(lines, field.View())
	}
	if m.form.error != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.form.error))
	}
	lines = append(lines, "", mutedStyle.Render("Tab/Shift+Tab move  Enter save  Esc cancel"))
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func (m Model) helpView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(fmt.Sprintf("%s\n\n1 Tasks    2 Journal    3 Today    4 Trash\nj/k or arrows  move\na  add    e  edit    x  complete/reopen\np  toggle important    d  delete    u  restore\nr  refresh    ?/Esc  close help    q  quit\n\nCurrent mode: %s", activeStyle.Render("daylog help"), []string{"Tasks", "Journal", "Today", "Trash"}[m.mode]))
}
