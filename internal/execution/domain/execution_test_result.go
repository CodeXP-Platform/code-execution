package domain

type ExecutionTestResult struct {
	ID              string
	ExecutionJobID  string
	TestID          string
	Passed          bool
	IsHidden        bool
	InputHash       string
	ActualOutput    string
	ExpectedOutput  string
	ErrorMessage    *string
	ExecutionTimeMs int
}
