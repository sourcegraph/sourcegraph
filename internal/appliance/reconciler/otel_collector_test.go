package reconciler

func (suite *ApplianceTestSuite) TestDeployOtelCollector() {
	for _, tc := range []struct {
		name string
	}{
		{name: "otel-collector/default"},
	} {
		suite.Run(tc.name, func() {
			namespace := suite.createConfigMapAndAwaitReconciliation(tc.name)
			suite.makeGoldenAssertions(namespace, tc.name)
		})
	}
}
