package reconciler

func (suite *ApplianceTestSuite) TestDeployGrafana() {
	for _, tc := range []struct {
		name string
	}{
		{name: "grafana/default"},
	} {
		suite.Run(tc.name, func() {
			namespace := suite.createConfigMapAndAwaitReconciliation(tc.name)
			suite.makeGoldenAssertions(namespace, tc.name)
		})
	}
}
