package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/db/dbconn"

	_ "github.com/lib/pq"
)

// This script generates markdown formatted output containing descriptions of
// the current dabase schema, obtained from postgres. The correct PGHOST,
// PGPORT, PGUSER etc. env variables must be set to run this script.
//
// First CLI argument is an optional filename to write the output to.
func generate(log *log.Logger) (string, error) {
	const dbname = "schemadoc-gen-temp"

	var (
		dataSource string
		run        func(cmd ...string) (string, error)
	)
	// If we are using pg9.6 use it locally since it is faster (CI \o/)
	if out, _ := exec.Command("pg_config", "--version").CombinedOutput(); bytes.Contains(out, []byte("PostgreSQL 9.6")) {
		dataSource = "dbname=" + dbname
		run = func(cmd ...string) (string, error) {
			c := exec.Command(cmd[0], cmd[1:]...)
			c.Stderr = log.Writer()
			out, err := c.Output()
			return string(out), err
		}
		_ = exec.Command("dropdb", dbname).Run()
		defer exec.Command("dropdb", dbname).Run()
	} else {
		log.Printf("Running PostgreSQL 9.6 in docker since local version is %s", strings.TrimSpace(string(out)))
		_ = exec.Command("docker", "rm", "--force", dbname).Run()
		server := exec.Command("docker", "run", "--rm", "--name", dbname, "-p", "5433:5432", "postgres:9.6")
		if err := server.Start(); err != nil {
			return "", err
		}

		defer func() {
			_ = server.Process.Kill()
			_ = exec.Command("docker", "kill", dbname).Run()
			_ = server.Wait()
		}()

		time.Sleep(1 * time.Second)
		dataSource = "postgres://postgres@localhost:5433/postgres?dbname=" + dbname
		run = func(cmd ...string) (string, error) {
			cmd = append([]string{"exec", "-u", "postgres", dbname}, cmd...)
			c := exec.Command("docker", cmd...)
			c.Stderr = log.Writer()
			out, err := c.Output()
			return string(out), err
		}

		attempts := 0
		for {
			attempts++
			if err := exec.Command("pg_isready", "-U", "postgres", "-d", dbname, "-h", "localhost", "-p", "5433").Run(); err == nil {
				break
			} else if attempts > 30 {
				return "", fmt.Errorf("gave up waiting for pg_isready: %w", err)
			}
			time.Sleep(time.Second)
		}
	}

	if out, err := run("createdb", dbname); err != nil {
		return "", fmt.Errorf("createdb: %s: %w", out, err)
	}

	if err := dbconn.ConnectToDB(dataSource); err != nil {
		return "", fmt.Errorf("ConnectToDB: %w", err)
	}

	db, err := dbconn.Open(dataSource)
	if err != nil {
		return "", fmt.Errorf("Open: %w", err)
	}

	// Query names of all public tables.
	rows, err := db.Query(`
SELECT table_name
FROM information_schema.tables
WHERE table_schema='public' AND table_type='BASE TABLE';
	`)
	if err != nil {
		return "", fmt.Errorf("Query: %w", err)
	}
	tables := []string{}
	defer rows.Close()
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			return "", fmt.Errorf("rows.Scan: %w", err)
		}
		tables = append(tables, name)
	}
	if err = rows.Err(); err != nil {
		return "", fmt.Errorf("rows.Err: %w", err)
	}

	docs := []string{}
	for _, table := range tables {
		// Get postgres "describe table" output.
		log.Println("describe", table)
		out, err := run("psql", "-X", "--quiet", "--dbname", dbname, "-c", fmt.Sprintf("\\d %s", table))
		if err != nil {
			return "", fmt.Errorf("describe %s failed: %w", table, err)
		}

		lines := strings.Split(string(out), "\n")
		doc := "# " + strings.TrimSpace(lines[0]) + "\n"
		doc += "```\n" + strings.Join(lines[1:], "\n") + "```\n"
		docs = append(docs, doc)
	}
	sort.Strings(docs)

	return strings.Join(docs, "\n"), nil
}

func main() {
	out, err := generate(log.New(os.Stderr, "", log.LstdFlags))
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		ioutil.WriteFile(os.Args[1], []byte(out), 0644)
	} else {
		fmt.Print(out)
	}
}
