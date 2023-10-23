package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mbndr/figlet4go"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

var items []list.Item

type MainModel struct {
	list     list.Model
	selected string
}

func (m MainModel) Init() tea.Cmd {
	return nil
}
func handleSelection(m MainModel) tea.Model {
	switch m.list.SelectedItem().FilterValue() {
	case "Payment":
		m := NewPaymentModel()
		return m
	case "Wallet":
		m := NewWalletModel()
		return m
	case "Pod":
		m := NewPodModel()
		return m
	}
	return m
}
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			m := handleSelection(m)
			return m, tea.ClearScreen

		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m MainModel) View() string {
	return docStyle.Render(m.list.View())
}

func NewMainModel() MainModel {
	items = []list.Item{
		item{title: "Payment", desc: "Operations Related to payment Channel"},
		item{title: "Pod", desc: "Operations Related to pod Managment"},
		item{title: "Wallet", desc: "Operations Related to Wallet managment"},
	}

	m := MainModel{list: list.New(items, list.NewDefaultDelegate(), 40, 50)}
	m.list.Title = "Trusted Pods"
	return m
}

func main() {
	m := NewMainModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func banner() string {

	ascii := figlet4go.NewAsciiRender()
	renderStr, _ := ascii.Render("Trusted Pods")
	return renderStr
}
