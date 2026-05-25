package picker

import "strings"

func (m PickerModel) viewTemplateSelect() string {
	var b strings.Builder

	label := m.styles.Accent.Bold(true).Render("  Select Template")
	b.WriteString(label + "\n\n")

	if len(m.templates) == 0 {
		b.WriteString(m.styles.Muted.Render("  No templates available") + "\n")
		return b.String()
	}

	for i, tmpl := range m.templates {
		selected := i == m.templateCursor

		cursor := "  "
		if selected {
			cursor = m.styles.Accent.Render("▸ ")
		}

		nameStyle := m.styles.Normal.Bold(true)
		if selected {
			nameStyle = m.styles.Accent.Bold(true)
		}

		line := "  " + cursor + nameStyle.Render(tmpl.Name)
		if tmpl.Description != "" {
			line += "  " + m.styles.Dim.Render(tmpl.Description)
		}
		if len(tmpl.Windows) > 0 {
			winNames := make([]string, 0, len(tmpl.Windows))
			for _, w := range tmpl.Windows {
				winNames = append(winNames, w.Name)
			}
			line += "  " + m.styles.Dim.Render("["+strings.Join(winNames, ", ")+"]")
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + m.styles.Dim.Render("  enter:select  esc:cancel") + "\n")
	return b.String()
}

func (m PickerModel) viewTemplateNameInput() string {
	var b strings.Builder

	label := m.styles.Accent.Bold(true).Render("  New from Template")
	tmplName := m.styles.Info.Render(m.selectedTemplate)
	b.WriteString(label + "  " + tmplName + "\n\n")

	prompt := m.styles.Accent.Render("  name ▸ ")
	b.WriteString(prompt + m.nameInput.View() + "\n")
	b.WriteString("\n" + m.styles.Dim.Render("  enter:create  esc:back") + "\n")

	return b.String()
}
