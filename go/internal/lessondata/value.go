package lessondata

// Kind is the JSON type of a parsed literal.
type Kind uint8

const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

// Value is one literal. Object key order is insertion order; JSON writers
// apply JavaScript's integer-index ordering on the way out.
type Value struct {
	Kind Kind
	Bool bool
	Num  float64
	Str  string // WTF-8, one UTF-16 code unit per scalar or surrogate
	Arr  []Value
	Obj  *Obj
}

// Obj is an ordered object. Duplicate keys keep their first position.
type Obj struct {
	Keys []string
	Vals []Value
}

func (o *Obj) set(key string, v Value) {
	for i, k := range o.Keys {
		if k == key {
			o.Vals[i] = v
			return
		}
	}
	o.Keys = append(o.Keys, key)
	o.Vals = append(o.Vals, v)
}

// Document is the set of window.NAME bindings in source order. Assigning the
// same name again updates the value and leaves the name where it first appeared.
type Document struct {
	Names  []string
	Values []Value
}

func (d *Document) set(name string, v Value) {
	for i, n := range d.Names {
		if n == name {
			d.Values[i] = v
			return
		}
	}
	d.Names = append(d.Names, name)
	d.Values = append(d.Values, v)
}

func (d *Document) get(name string) (Value, bool) {
	for i, n := range d.Names {
		if n == name {
			return d.Values[i], true
		}
	}
	return Value{}, false
}

func nullValue() Value { return Value{Kind: Null} }

func boolValue(b bool) Value { return Value{Kind: Bool, Bool: b} }

func numValue(n float64) Value { return Value{Kind: Number, Num: n} }

func strValue(s string) Value { return Value{Kind: String, Str: s} }

func arrValue(a []Value) Value { return Value{Kind: Array, Arr: a} }

func objValue(o *Obj) Value { return Value{Kind: Object, Obj: o} }
