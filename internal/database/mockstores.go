package database

var Mocks MockStores

// MockStores has a field for each store interface with the concrete mock type (to obviate the need for tedious type assertions in test code).
type MockStores struct {
	AccessTokens MockAccessTokens

	Repos           MockRepos
	Namespaces      MockNamespaces
	Orgs            MockOrgs
	OrgMembers      MockOrgMembers
	SavedSearches   MockSavedSearches
	Settings        MockSettings
	Users           MockUsers
	UserCredentials MockUserCredentials
	UserEmails      MockUserEmails
	UserPublicRepos MockUserPublicRepos

	Phabricator MockPhabricator

	ExternalAccounts MockExternalAccounts

	OrgInvitations MockOrgInvitations

	ExternalServices MockExternalServices

	Authz MockAuthz

	EventLogs MockEventLogs
}
