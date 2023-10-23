package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PaymentModel struct {
	list list.Model
}

func NewPaymentModel() PaymentModel {
	items = []list.Item{
		item{title: "Create", desc: "Create Payment Channel"},
		item{title: "List", desc: "List Available channels"},
		item{title: "Unlock", desc: "Unlock Funds From Channel"},
		item{title: "Deposit", desc: "Deposit more Funds From Channel"},
	}
	m := PaymentModel{list: list.New(items, list.NewDefaultDelegate(), 40, 50)}
	m.list.Title = "Payment Channel Operations"
	return m
}

func (m PaymentModel) Init() tea.Cmd {
	return nil
}

var titleStyle lipgloss.Style

func (m PaymentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "backspace":
			m := NewMainModel()
			return m, tea.ClearScreen
		case "enter":
			m := handlePaymentSelection(m)
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

func (m PaymentModel) View() string {
	titleStyle = lipgloss.NewStyle().MarginLeft(0).Background(lipgloss.Color("#FF5733")).Bold(true)
	m.list.Title = "Payment Channel Operations"
	m.list.Styles.Title = titleStyle
	return docStyle.Render(m.list.View())
}

func handlePaymentSelection(m PaymentModel) tea.Model {
	return NewInputModel()
}
