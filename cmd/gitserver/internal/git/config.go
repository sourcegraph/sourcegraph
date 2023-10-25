package git

import (
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/config"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ConfigGet returns a git config setting.
func ConfigGet(rcf *wrexec.RecordingCommandFactory, reposDir string, dir common.GitDir, key string) (string, error) {
	r, err := git.PlainOpen(dir.Path())
	if err != nil {
		return "", err
	}

	section, subsection, field, err := splitSections(key)
	if err != nil {
		return "", err
	}

	cfg, err := r.Config()
	if err != nil {
		return "", err
	}

	sec := cfg.Raw.Section(section)
	if subsection != config.NoSubsection {
		return sec.Subsection(subsection).Option(field), nil
	}

	return cfg.Raw.Section(section).Option(field), nil

	// cmd := exec.Command("git", "config", "--get", key)
	// dir.Set(cmd)
	// wrappedCmd := rcf.WrapWithRepoName(context.Background(), log.NoOp(), gitserverfs.RepoNameFromDir(reposDir, dir), cmd)
	// out, err := wrappedCmd.Output()
	// if err != nil {
	// 	// Exit code 1 means the key is not set.
	// 	var e *exec.ExitError
	// 	if errors.As(err, &e) && e.Sys().(syscall.WaitStatus).ExitStatus() == 1 {
	// 		return "", nil
	// 	}
	// 	return "", errors.Wrapf(executil.WrapCmdError(cmd, err), "failed to get git config %s", key)
	// }
	// return strings.TrimSpace(string(out)), nil
}

// ConfigSet sets a git config value.
//
// Warning: This operation may fail with error code 255 when another routine or
// git itself currently hold a lock on the config file!
func ConfigSet(rcf *wrexec.RecordingCommandFactory, reposDir string, dir common.GitDir, key, value string) error {
	r, err := git.PlainOpen(dir.Path())
	if err != nil {
		return err
	}

	section, subsection, field, err := splitSections(key)
	if err != nil {
		return err
	}

	cfg, err := r.Config()
	if err != nil {
		return err
	}

	cfg.Raw.SetOption(section, subsection, field, value)

	return r.SetConfig(cfg)

	// cmd := exec.Command("git", "config", key, value)
	// dir.Set(cmd)
	// wrappedCmd := rcf.WrapWithRepoName(context.Background(), log.NoOp(), gitserverfs.RepoNameFromDir(reposDir, dir), cmd)
	// out, err := wrappedCmd.CombinedOutput()
	// if err != nil {
	// 	return errors.Wrapf(executil.WrapCmdError(cmd, err), "failed to set git config %s: %s", key, string(out))
	// }
	// return nil
}

// ConfigSet removes all instances of a key in git config.
//
// Warning: This operation may fail with error code 255 when another routine or
// git itself currently hold a lock on the config file!
func ConfigUnset(rcf *wrexec.RecordingCommandFactory, reposDir string, dir common.GitDir, key string) error {
	r, err := git.PlainOpen(dir.Path())
	if err != nil {
		return err
	}

	section, subsection, field, err := splitSections(key)
	if err != nil {
		return err
	}

	cfg, err := r.Config()
	if err != nil {
		return err
	}

	if subsection != config.NoSubsection {
		cfg.Raw.Section(section).Subsection(subsection).RemoveOption(field)
	} else {
		cfg.Raw.Section(section).RemoveOption(field)
	}

	return r.SetConfig(cfg)

	// cmd := exec.Command("git", "config", "--unset-all", key)
	// dir.Set(cmd)
	// wrappedCmd := rcf.WrapWithRepoName(context.Background(), log.NoOp(), gitserverfs.RepoNameFromDir(reposDir, dir), cmd)
	// out, err := wrappedCmd.CombinedOutput()
	// if err != nil {
	// 	// Exit code 5 means the key is not set.
	// 	var e *exec.ExitError
	// 	if errors.As(err, &e) && e.Sys().(syscall.WaitStatus).ExitStatus() == 5 {
	// 		return nil
	// 	}
	// 	return errors.Wrapf(executil.WrapCmdError(cmd, err), "failed to unset git config %s: %s", key, string(out))
	// }
	// return nil
}

func splitSections(key string) (section, subsection, field string, err error) {
	s := strings.Split(key, ".")
	if len(s) < 2 {
		return "", "", "", errors.New("key must contain section and field separated by '.'")
	}
	if len(s) > 3 {
		return "", "", "", errors.New("key must contain at most section, one subsection, and field separated by '.'")
	}

	section, field = s[0], s[len(s)-1]
	if len(s) == 3 {
		subsection = s[1]
	} else {
		subsection = config.NoSubsection
	}

	return section, subsection, field, nil
}
