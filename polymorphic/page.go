package polymorphic

func init() {
	RegisterType[PolymorphicPage]()
}

type PolymorphicPage struct {
	Envelopes []*Envelope `json:"envelopes,omitempty"`
	Prev      string      `json:"prev,omitempty"`
	Next      string      `json:"next,omitempty"`
}

func (obj *PolymorphicPage) GetDiscriminator() string {
	return "mesh://pages/page"
}

type Page[T Polymorphic] struct {
	Models []T    `json:"models,omitempty"`
	Prev   string `json:"prev,omitempty"`
	Next   string `json:"next,omitempty"`
}

func ToPage[T Polymorphic](obj *PolymorphicPage) *Page[T] {
	models := make([]T, 0, len(obj.Envelopes))
	for _, envelope := range obj.Envelopes {
		if x, ok := envelope.Content.(T); ok {
			models = append(models, x)
		}
	}

	page := &Page[T]{
		Prev:   obj.Prev,
		Next:   obj.Next,
		Models: models,
	}
	return page
}
