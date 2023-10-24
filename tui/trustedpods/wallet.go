package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type WalletModel struct {
	list list.Model
}

func NewWalletModel() WalletModel {
	items = []list.Item{
		item{title: "Create Account", desc: "Create New Account"},
		item{title: "Import Account", desc: "Import Account using private key"},
		item{title: "Select Account", desc: "Select a Diffrent account"},
	}
	m := WalletModel{list: list.New(items, list.NewDefaultDelegate(), 40, 50)}
	m.list.Title = "Wallet Managment Operations"
	return m
}

func (m WalletModel) Init() tea.Cmd {
	return nil
}

func (m WalletModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "backspace":
			m := NewMainModel()
			return m, nil
		case "enter":
			handleWalletSelection(m)
			return m, nil
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m WalletModel) View() string {

	titleStyle = lipgloss.NewStyle().MarginLeft(0).Background(lipgloss.Color("#008080")).Bold(true)
	m.list.Title = "Wallet Managment Operations"
	m.list.Styles.Title = titleStyle
	return docStyle.Render(m.list.View())
}

func handleWalletSelection(m WalletModel) {

}
