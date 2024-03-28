# Codehost Testing Library

This library makes it easier to setup Codehost resources in a predicatable and reproducible manner. It accomplishes this by introducing the concept of a scenario.

A scenario describes the state a collection of related resources on a codehost must be in. A collection of resources can at it's most basic just be an Organisation or at it's most complex be an Organisation with various teams and repositories with varying permissions.

Supported Codehosts:

- GitHub

## Configuration

The following configuration is required by this library

```json
{
  "github": {
    "url": "https://path.codehost.org",
    "adminUser": "dude",
    "password": "whereismycar",
    "token": "do_not_leak_me"
  },
  "sourcegraph": {
    "url": "https://path.sourcegraph.org",
    "user": "boi",
    "password": "towers_of_hanoi",
    "token": "do_not_leak_me_plz"
  }
}
```

Additionally, the GitHub token is **required** to have the following scopes:

- admin:enterprise
- delete_repo
- repo
- site_admin
- user
- write:org

To load your configuration you can use `config.FromFile` as per the snippet below.

```golang
	cfg, err := config.FromFile("config.json")
	if err != nil {
		t.Fatalf("error loading scenario config: %v\n", err)
	}
```

## Usage

We first need to create a scenario. A scenario exposes various methods to add or alter the scenario. In the below snippet we create a scenario that describes the following:

- One Organization
- One normal user
- One Admin
- Two teams. One team with the normal user and the other team with the Admin
- Two repositories are forked. One repository is public while the other is private and only accessible by the private team.

```golang
	scenario, err := scenario.NewGitHubScenario(t, *cfg)
	if err != nil {
		t.Fatalf("error creating scenario: %v\n", err)
	}
	org := scenario.CreateOrg("tst-org")
	user := scenario.CreateUser("tst-user")
	admin := scenario.GetAdmin()

	org.AllowPrivateForks()
	team := org.CreateTeam("team-1")
	team.AddUser(user)
	adminTeam := org.CreateTeam("team-admin")
	adminTeam.AddUser(admin)

	publicRepo := org.CreateRepoFork("sgtest/go-diff")
	publicRepo.AddTeam(team)
	privateRepo := org.CreateRepoFork("sgtest/private")
	privateRepo.AddTeam(adminTeam)
```

Adding a resource to the scenario does not immediately create or alter it. Instead, when adding a resource to the scenario, what you are actually doing is telling the library that "I want this resource to exist with this particular make up". The scenario keeps track of how all these resources that should be created and altered as **Actions** to be applied sequentially. Thus when calling `scenario.CreateOrg` the Org isn't immediately created, instead, an action is added that \_will create it when the scenario is applied.

We can see what actions our scenario **plans** to apply by calling `Plan` as per the below snippet. `Plan` returns a string that prints out the action names that will be applied by the scenario.

```golang
	fmt.Println(scenario.Plan())
```

As was mentioned before, since we haven't applied the scenario, nothing exists yet on the Codehost. To create the resources described by the scenario `Apply` has to be called. In the snippet below the verbosity of the scenario is increased so that we can see how and when the actions are applied.

```
	scenario.SetVerbose()
	if err := scenario.Apply(context.Background()); err != nil {
		t.Fatalf("error applying scenario: %v", err)
	}
```

When `Apply` is called on the scenario, the scenario will not only apply all the actions but will also register a corresponding cleanup method with `testing.T` to teardown all resources that will be created by this scenario. Thus, if anything fails, the resources that _have been_ created up and till that point, will be properly
cleaned up.

After the scenario has been successfully been applied, the corresponding Codehost resource can be retrieved by calling the `Get()` on the scenario resource. For example on the below snippet, the `github.Organization` is retrieved.

```golang
  ghOrg, err := org.Get(ctx)
  if err != nil {
    t.Fatalf("failed to get Organzation: %v", err)
  }
```

**IMPORTANT** Calling `Get()` on any scenario resource before the scenario has been applied will result in an error. **Get() will only return the Codehost resource if the scenario has been applied**.
Hello World
