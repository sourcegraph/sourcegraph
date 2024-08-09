package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run script.go SAMS_ID SAMS_SESSION_COOKIE")
		os.Exit(1)
	}

	samsID := os.Args[1]
	samsSessionCookie := os.Args[2] // Get this from your browser's cookies.
	sscHost := "https://accounts.sourcegraph.com"
	teamID := "018da9a6-ec3d-7ab9-9468-44fef461607a" // ID of the unofficial Sourcegraph Cody Pro team.

	fmt.Println("Creating Cody user account...")
	createUserAccount(sscHost, samsID, samsSessionCookie)

	fmt.Println("Adding member to PLG team...")
	addMemberToTeam(sscHost, teamID, samsID, samsSessionCookie)
}

// createUserAccount creates a user account in SSC. This step is required for anybody who
// hasn't visited https://accounts.sourcegraph.com/cody before.
func createUserAccount(host, accountID, cookie string) {
	req, err := http.NewRequest("PUT", host+"/cody/api/rest/admin/user/"+accountID, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Add("Cookie", "accounts_session_v2="+cookie)
	req.Header.Add("Content-Length", "0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}
	fmt.Println("Response status for creating user account:", resp.Status, "-", string(body))
}

// addMemberToTeam adds a member to the Sourcegraph team in SSC.
func addMemberToTeam(host, teamID, accountID, cookie string) {
	req, err := http.NewRequest("PUT", host+"/cody/api/rest/admin/team/"+teamID+"/members/"+accountID, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Add("Cookie", "accounts_session_v2="+cookie)
	req.Header.Add("Content-Length", "0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("Response status for adding member to team:", resp.Status, "-", string(body))
}
