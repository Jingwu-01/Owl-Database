package patcher

type Patch struct {
	Op    string
	Path  string
	value map[string]interface{}
}

type PatchVisitor struct {
	patches []Patch
}

func New(patches []Patch) PatchVisitor {
	return PatchVisitor{patches}
}

func (p PatchVisitor) Map(m map[string]any) (bool, error) {
	return false, nil
}

func (p PatchVisitor) Slice(s []any) (bool, error) {
	return false, nil
}

func (p PatchVisitor) Bool(b bool) (bool, error) {
	return false, nil
}

func (p PatchVisitor) Float64(f float64) (bool, error) {
	return false, nil
}

func (p PatchVisitor) String(string) (bool, error) {
	return false, nil
}

func (p PatchVisitor) Null() (bool, error) {
	return false, nil
}
