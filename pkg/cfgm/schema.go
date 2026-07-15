package cfgm

import "reflect"

type Schema struct {
	model           *schemaModel
	codecs          map[reflect.Type]valueCodec
	expandTemplates bool
}

type Field struct {
	Path string
	Type reflect.Type
	Desc string
}

func (s Schema) Fields() []Field {
	if s.model == nil {
		return nil
	}
	fields := make([]Field, len(s.model.fields))
	for index, field := range s.model.fields {
		fields[index] = Field{Path: field.path, Type: field.typ, Desc: field.desc}
	}
	return fields
}

func (s Schema) Has(path string) bool {
	return s.model != nil && s.model.hasPath(cleanConfigPath(path))
}
