package op

type OperatorType int

const (
	UndefinedType OperatorType = iota
	AndType
	AnyType
	CaptureType
	EOLType
	NotType
	OneOrMoreType
	OptionalType
	OrType
	RuneRangeType
	SpaceType
	ZeroOrMoreType
)
