package template

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*.html
var embeddedTemplates embed.FS

type Renderer interface {
	Render(name string, data any) (string, error)
}

type HTMLRenderer struct {
	templates map[string]*template.Template
}

func NewRenderer() Renderer {
	return &HTMLRenderer{
		templates: loadTemplates(),
	}
}

func (r *HTMLRenderer) Render(name string, data any) (string, error) {
	t, ok := r.templates[name]
	if !ok {
		return "", fmt.Errorf("email template %q not found", name)
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render email template %q: %w", name, err)
	}

	return buf.String(), nil
}

func loadTemplates() map[string]*template.Template {
	names := []string{
		"booking_online.html",
		"booking_offline.html",
		"cancellation.html",
		"rescheduling.html",
	}

	result := make(map[string]*template.Template, len(names))

	for _, name := range names {
		content, err := embeddedTemplates.ReadFile("templates/" + name)
		if err != nil {
			panic(fmt.Sprintf("booking mailer: read template %s: %v", name, err))
		}

		result[name] = template.Must(template.New(name).Parse(string(content)))
	}

	return result
}
