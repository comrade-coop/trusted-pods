package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PodModel struct {
	list list.Model
}

func NewPodModel() PodModel {
	items = []list.Item{
		item{title: "Upload", desc: "Upload Pod to ipfs"},
		item{title: "Deploy", desc: "Upload Pod to provider"},
		item{title: "List", desc: "List all Pods"},
	}
	m := PodModel{list: list.New(items, list.NewDefaultDelegate(), 40, 50)}
	m.list.Title = "Payment Channel Operations"
	return m
}

func (m PodModel) Init() tea.Cmd {
	return nil
}

func (m PodModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "backspace":
			m := NewMainModel()
			return m, tea.ClearScreen
		case "enter":
			handlePodSelection(m)
			return m, tea.EnterAltScreen
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}
func (m PodModel) View() string {

	titleStyle = lipgloss.NewStyle().MarginLeft(0).Background(lipgloss.Color("#702963")).Bold(true)
	m.list.Title = "Pod Managment"
	m.list.Styles.Title = titleStyle
	return docStyle.Render(m.list.View())
}

func handlePodSelection(m PodModel) {

}
