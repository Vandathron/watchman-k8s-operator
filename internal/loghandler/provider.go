package loghandler

import "fmt"

type Provider interface {
	Log(resource, action, namespace string, data Data)
}

type Data struct {
	fields map[string]string
}

func (d *Data) AddField(field, value string) {
	if d.fields == nil {
		d.fields = map[string]string{}
	}
	d.fields[field] = fmt.Sprintf("Changed to %s", value)
}
