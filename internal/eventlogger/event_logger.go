package eventlogger

// List of events that don't meet the criteria of "active" usage of Sourcegraph.
// These are mostly actions taken by signed-out users.
var NonActiveUserEvents = []string{
	// V2 events
	"auth.signIn.view",
	"auth.signUp.view",
	"signOut.attempted",
	"signOut.failed",
	"signOut.succeeded",
	"signIn.attempted",
	"signIn.failed",
	"signIn.succeeded",
	"externalAuthSignup.succeeded",
	"externalAuthSignup.failed",

	// V1 events, deprecated
	"ViewSignIn",
	"ViewSignUp",
	"SignOutAttempted",
	"SignOutFailed",
	"SignOutSucceeded",
	"SignInAttempted",
	"SignInFailed",
	"SignInSucceeded",
	"PasswordResetRequested",
	"PasswordRandomized",
	"PasswordChanged",
	"EmailVerified",
	"ExternalAuthSignupFailed",
	"ExternalAuthSignupSucceeded",
	"CodyVSCodeExtension:CodySavedLogin:executed",
	"cody.characters.flush",
	"cody.characters",
}

// List of events that shouldn't be logged in local (Postgres) databases.
// These events are high volume and cause a lot of noise and pressure on the backend.
// Since we only need them for debugging, we don't need to route them to custom instances.
var OnlyLogRemotelyEvents = []string{
	"CodyVSCodeExtension:completion:started",
	"CodyVSCodeExtension:completion:networkRequestStarted",
}
