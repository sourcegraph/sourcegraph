package operator

type K8sManager interface {
	Status() *status
	Install(version string) error
	Upgrade(version string) error
}

func New() K8sManager {
	return &manager{}
}

type status struct {
	Stage          Stage    `json:"stage"`
	CurrentVersion *string  `json:"version"`     // current version, nil if not installed
	NextVersion    *string  `json:"nextVersion"` // version being installed/upgraded nil if not being installed/upgraded
	Tasks          []Task   `json:"tasks"`
	Errors         []string `json:"errors"`
}

type Stage string

const (
	StageUnknown         Stage = "unknown"
	StageIdle            Stage = "idle"
	StageInstall         Stage = "install"
	StageInstalling      Stage = "installing"
	StageUpgrading       Stage = "upgrading"
	StageWaitingForAdmin Stage = "wait-for-admin"
	StageRefresh         Stage = "refresh"
)

type manager struct{}

// Asks the Operator to kick off a new installation of the specified version.
//
// Returns an error if the installation was not successful,
// if the version is not supported, or a version is already installed.
//
// Once the request is accepted, the status can be tracked via the Status() method.
func (*manager) Install(version string) error {
	panic("unimplemented")
}

// Asks the Operator to upgrade to the specified version.
//
// Returns an error if the upgrade was not successful,
// if the version is not supported, or if there's no existing version installed.
//
// Once the request is accepted, the status can be tracked via the Status() method.
func (*manager) Upgrade(version string) error {
	panic("unimplemented")
}

// Returns the current status of the Operator.
func (*manager) Status() *status {
	panic("unimplemented")
}
