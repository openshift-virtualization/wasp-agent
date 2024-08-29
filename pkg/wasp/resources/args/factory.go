package args

// FactoryArgs contains the required parameters to generate all namespaced resources
type FactoryArgs struct {
	SwapUtilizationThresholdFactor  string `required:"true" split_words:"true"`
	MaxAverageSwapInPagesPerSecond  string `required:"true" split_words:"true"`
	MaxAverageSwapOutPagesPerSecond string `required:"true" split_words:"true"`
	OperatorVersion                 string `required:"true" split_words:"true"`
	WaspImage                       string `required:"true" split_words:"true"`
	DeployClusterResources          string `required:"true" split_words:"true"`
	DeployPrometheusRule            string `required:"true" split_words:"true"`
	Verbosity                       string `required:"true"`
	AverageWindowSizeSeconds        string `required:"true"`
	PullPolicy                      string `required:"true" split_words:"true"`
	Namespace                       string
}
