package vcssyncer

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/unpack"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/jvmpackages/coursier"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

const (
	// DO NOT CHANGE. This timestamp needs to be stable so that JVM package
	// repos consistently produce the same git revhash. Sourcegraph URLs
	// can optionally include this hash, so changing the timestamp (and hence
	// hashes) will cause existing links to JVM package repos to return 404s.
	stableGitCommitDate = "Thu Apr 8 14:24:52 2021 +0200"

	jvmMajorVersion0 = 44
)

func NewJVMPackagesSyncer(connection *schema.JVMPackagesConnection, svc *dependencies.Service, getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error), cacheDir string, fs gitserverfs.FS) VCSSyncer {
	placeholder, err := reposource.ParseMavenVersionedPackage("com.sourcegraph:sourcegraph:1.0.0")
	if err != nil {
		panic(fmt.Sprintf("expected placeholder package to parse but got %v", err))
	}

	chandle := coursier.NewCoursierHandle(observation.NewContext(log.Scoped("gitserver.jvmsyncer")), cacheDir)

	return &vcsPackagesSyncer{
		logger:      log.Scoped("JVMPackagesSyncer"),
		typ:         "jvm_packages",
		scheme:      dependencies.JVMPackagesScheme,
		placeholder: placeholder,
		svc:         svc,
		configDeps:  connection.Maven.Dependencies,
		fs:          fs,
		source: &jvmPackagesSyncer{
			coursier: chandle,
			config:   connection,
			fetch:    chandle.FetchSources,
		},
		getRemoteURLSource: getRemoteURLSource,
	}
}

type jvmPackagesSyncer struct {
	coursier *coursier.CoursierHandle
	config   *schema.JVMPackagesConnection
	fetch    func(ctx context.Context, config *schema.JVMPackagesConnection, dependency *reposource.MavenVersionedPackage) (sourceCodeJarPath string, err error)
}

func (jvmPackagesSyncer) ParseVersionedPackageFromNameAndVersion(name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	return reposource.ParseMavenVersionedPackage(string(name) + ":" + version)
}

func (jvmPackagesSyncer) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseMavenVersionedPackage(dep)
}

func (jvmPackagesSyncer) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return reposource.ParseMavenPackageFromName(name)
}

func (jvmPackagesSyncer) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return reposource.ParseMavenPackageFromRepoName(repoName)
}

func (s *jvmPackagesSyncer) Download(ctx context.Context, dir string, dep reposource.VersionedPackage) error {
	mavenDep := dep.(*reposource.MavenVersionedPackage)
	sourceCodeJarPath, err := s.fetch(ctx, s.config, mavenDep)
	if err != nil {
		return notFoundError{errors.Errorf("%s not found", dep)}
	}

	// commitJar creates a git commit in the given working directory that adds all the file contents of the given jar file.
	// A `*.jar` file works the same way as a `*.zip` file, it can even be uncompressed with the `unzip` command-line tool.
	if err := unzipJarFile(sourceCodeJarPath, dir); err != nil {
		return errors.Wrapf(err, "failed to unzip jar file for %s to %v", dep, sourceCodeJarPath)
	}

	file, err := os.Create(filepath.Join(dir, "lsif-java.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	jvmVersion, err := s.inferJVMVersionFromByteCode(ctx, mavenDep)
	if err != nil {
		return err
	}

	// See [NOTE: LSIF-config-json] for details on why we use this JSON file.
	jsonContents, err := json.Marshal(&lsifJavaJSON{
		Kind:         mavenDep.MavenModule.LsifJavaKind(),
		JVM:          jvmVersion,
		Dependencies: mavenDep.LsifJavaDependencies(),
	})
	if err != nil {
		return err
	}

	_, err = file.Write(jsonContents)
	if err != nil {
		return err
	}

	return nil
}

func unzipJarFile(jarPath, destination string) (err error) {
	logger := log.Scoped("unzipJarFile")
	workDir := strings.TrimSuffix(destination, string(os.PathSeparator)) + string(os.PathSeparator)

	zipFile, err := os.ReadFile(jarPath)
	if err != nil {
		return errors.Wrap(err, "bad jvm package")
	}

	r := bytes.NewReader(zipFile)
	opts := unpack.Opts{
		SkipInvalid:    true,
		SkipDuplicates: true,
		Filter: func(path string, file fs.FileInfo) bool {
			size := file.Size()

			const sizeLimit = 15 * 1024 * 1024
			slogger := logger.With(
				log.String("path", file.Name()),
				log.Int64("size", size),
				log.Float64("limit", sizeLimit),
			)
			if size >= sizeLimit {
				slogger.Warn("skipping large file in JVM package")
				return false
			}

			malicious := isPotentiallyMaliciousFilepathInArchive(path, workDir)
			return !malicious
		},
	}

	err = unpack.Zip(r, int64(len(zipFile)), workDir, opts)

	if err != nil {
		return err
	}

	return nil
}

// inferJVMVersionFromByteCode returns the JVM version that was used to compile
// the bytecode in the given jar file.
func (s *jvmPackagesSyncer) inferJVMVersionFromByteCode(ctx context.Context,
	dependency *reposource.MavenVersionedPackage,
) (string, error) {
	if dependency.IsJDK() {
		return dependency.Version, nil
	}

	byteCodeJarPath, err := s.coursier.FetchByteCode(ctx, s.config, dependency)
	if err != nil {
		return "", err
	}
	majorVersionString, err := classFileMajorVersion(byteCodeJarPath)
	if err != nil {
		return "", err
	}
	majorVersion, err := strconv.Atoi(majorVersionString)
	if err != nil {
		return "", errors.Wrapf(err, "failed to convert string %s to int", majorVersion)
	}

	// Java 1.1 (aka "Java 1") has major version 45 and Java 8 has major
	// version 52. To go from the major version of Java version we subtract
	// 44.
	jvmVersion := majorVersion - jvmMajorVersion0

	// The motivation to round the JVM version to the nearst stable release
	// is so that we reduce the number of JDKs on sourcegraph.com. By having
	// fewer JDK versions, features like "find references" will return
	// aggregated results for non-LTS releases.
	roundedJvmVersion := roundJVMVersionToNearestStableVersion(jvmVersion)

	return strconv.Itoa(roundedJvmVersion), nil
}

// roundJVMVersionToNearestStableVersion returns the oldest stable JVM version
// that is compatible with the given version. Java uses a time-based release
// schedule since Java 11. A new major version is released every 6 month and
// every 6th release is an LTS release. This means that a new LTS release gets
// published every 3rd year.  See
// https://www.baeldung.com/java-time-based-releases for more details.  This
// method rounds up non-LTS versions to the nearest LTS release. For example, a
// library that's published for Java 10 should be indexed with Java 11.
func roundJVMVersionToNearestStableVersion(javaVersion int) int {
	if javaVersion <= 8 {
		return 8
	}
	if javaVersion <= 11 {
		return 11
	}
	if javaVersion <= 17 {
		return 17
	}
	// Version from the future, do not round up to the next stable release.
	return javaVersion
}

type lsifJavaJSON struct {
	Kind         string   `json:"kind"`
	JVM          string   `json:"jvm"`
	Dependencies []string `json:"dependencies"`
}

// classFileMajorVersion returns the "major" version of the first `*.class` file
// inside the given jar file. For example, a jar file for a Java 8 library has
// the major version 52.
func classFileMajorVersion(byteCodeJarPath string) (string, error) {
	file, err := os.OpenFile(byteCodeJarPath, os.O_RDONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := os.Stat(byteCodeJarPath)
	if err != nil {
		return "", err
	}
	zipReader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return "", errors.Wrapf(err, "failed to read jar file %s", byteCodeJarPath)
	}

	for _, zipEntry := range zipReader.File {
		if !strings.HasSuffix(zipEntry.Name, ".class") {
			continue
		}
		version, err := classFileEntryMajorVersion(byteCodeJarPath, zipEntry)
		if err != nil {
			return "", nil
		}
		if version == "" {
			continue // Not a classfile
		}
		return version, nil
	}

	// We didn't find any `*.class` files so we can use any Java version.
	// Maven don't have to contain classfiles, some artifacts like
	// 'io.smallrye:smallrye-health-ui:3.1.1' only contain HTML/css/png/js
	// files.
	return "8", nil
}

func classFileEntryMajorVersion(byteCodeJarPath string, zipEntry *zip.File) (string, error) {
	classFileReader, err := zipEntry.Open()
	if err != nil {
		return "", err
	}

	magicBytes := make([]byte, 8)
	read, err := classFileReader.Read(magicBytes)
	defer classFileReader.Close()
	if err != nil {
		return "", err
	}
	if read != len(magicBytes) {
		return "", errors.Errorf("failed to read 8 bytes from file %s", byteCodeJarPath)
	}

	// The structure of `*.class` files is documented here
	// https://docs.oracle.com/javase/specs/jvms/se16/html/jvms-4.html#jvms-4.1 and also
	// https://en.wikipedia.org/wiki/Java_class_file#General_layout
	// - Bytes 0-4 must match 0xcafebabe.
	// - Bytes 4-5 represent the uint16 formatted "minor" version.
	// - Bytes 5-6 represent the uint16 formatted "major" version.
	// We're only interested in the major version.
	var cafebabe uint32
	var minor uint16
	var major uint16
	buf := bytes.NewReader(magicBytes)
	if err := binary.Read(buf, binary.BigEndian, &cafebabe); err != nil {
		return "", errors.Wrap(err, "failed to read classfile header")
	}
	if cafebabe != 0xcafebabe {
		return "", nil // Not a classfile
	}
	if err := binary.Read(buf, binary.BigEndian, &minor); err != nil {
		return "", errors.Wrap(err, "failed to read classfile header")
	}
	if err := binary.Read(buf, binary.BigEndian, &major); err != nil {
		return "", errors.Wrap(err, "failed to read classfile header")
	}
	return strconv.Itoa(int(major)), nil
}
