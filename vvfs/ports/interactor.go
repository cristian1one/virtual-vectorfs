package ports

type Interactor interface {
	Output(message string)
	Warning(message string)
	Error(message string, err error)
	StartSpinner(message string)
	StopSpinner(success bool, message string)
}
