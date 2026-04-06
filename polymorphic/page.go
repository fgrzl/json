package polymorphic

func init() {
	registerDefaultType[PolymorphicPage]()
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

// ToPage projects obj.Envelopes into Page[T].Models by type assertion to T.
// Envelopes whose Content is not assignable to T are skipped (no error).
// Use this to filter a page that contains multiple polymorphic types down to
// a slice of a single type.
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
