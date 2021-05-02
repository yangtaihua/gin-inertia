package inertia

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
	"strings"
)

type inertiaCtxKeyType string

const inertiaCtxKey inertiaCtxKeyType = "inertia"

type Inertia struct {
	RootTemplate     *template.Template
	RootTemplateData P

	Version     string
	VersionFunc func() string
	shared      P
}

func Init(rootTemplate *template.Template) *Inertia {
	return &Inertia{
		RootTemplate: rootTemplate,
	}
}
func (i *Inertia) SetVersionFunc(f func() string) {
	i.VersionFunc = f
}

func (i *Inertia) ShareMulti(p P) {
	if i.shared == nil {
		i.shared = P{}
	}
	i.shared.merge(p)
}

func (i *Inertia) Share(prop string, value interface{}) {
	if i.shared == nil {
		i.shared = P{}
	}
	i.shared[prop] = value
}

type P map[string]interface{}

// merge merges two maps. On duplicates, if two maps merge recursively, replace with other's key otherwise.
func (p P) merge(other P) {
	for k, v := range other {
		existing, ok := p[k]
		if ok {
			existingP, ok1 := existing.(P)
			vP, ok2 := v.(P)
			if ok1 && ok2 {
				existingP.merge(vP)
			} else {
				p[k] = v
			}
		} else {
			p[k] = v
		}
	}
}

func (i *Inertia) getVersion() string {
	if i.VersionFunc != nil {
		return i.VersionFunc()
	}
	return i.Version
}

type page struct {
	Component string                 `json:"component"`
	Props     map[string]interface{} `json:"props"`
	URL       string                 `json:"url"`
	Version   string                 `json:"version"`
}

func (i *Inertia) render(c *gin.Context, componentName string, p P) {
	// merge shared and render pros into new P
	props := P{}
	props.merge(i.shared)
	props.merge(p)
	if only := strings.Split(c.GetHeader("X-Inertia-Partial-Data"), ","); len(only) != 0 && c.GetHeader("X-Inertia-Partial-Component") == componentName {
		newProps := make(map[string]interface{})
		for _, k := range only {
			if p, ok := props[k]; ok {
				newProps[k] = p
			}
		}
		props = newProps
	}

	// perform lazy evaluation
	for k, v := range props {
		switch v := v.(type) {
		case func() interface{}:
			props[k] = v()
		case func(r *http.Request) interface{}:
			props[k] = v(c.Request)
		case func(w http.ResponseWriter, r *http.Request) interface{}:
			props[k] = v(c.Writer, c.Request)
		}
	}

	page := page{
		Component: componentName,
		Props:     props,
		URL:       c.Request.RequestURI,
		Version:   i.getVersion(),
	}

	marshalled, err := json.Marshal(page)
	if err != nil {
		c.AbortWithStatus(500)
		return
	}

	if c.GetHeader("X-Inertia") != "" {

		c.Writer.Header().Set("Vary", "Accept")
		c.Writer.Header().Set("Vary", "Accept")
		c.Writer.Header().Set("Content-type", "application/json")
		c.Header("Vary", "Accept")
		c.Header("X-Inertia", "true")
		c.Header("Content-type", "application/json")
		c.Writer.Write(marshalled)
		return
	}

	err = i.RootTemplate.Execute(c.Writer, map[string]interface{}{
		"page": template.HTML(marshalled),
		"data": i.RootTemplateData,
	})
	if err != nil {
		c.AbortWithStatus(500)
		return
	}
}

func Render(c *gin.Context, componentName string, props P) {

	inertia, ok := c.Value(inertiaCtxKey).(*Inertia)
	if !ok {
		panic("[Inertia] No middleware configured.")
	}
	inertia.render(c, componentName, props)
}
