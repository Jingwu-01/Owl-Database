package authentication

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type test struct {
	r        *http.Request
	w        *httptest.ResponseRecorder
	expected string
	code     int
}

// authorization test (Note that login successful and logout successfull need to be tested through Swagger, because the token is randomly generated each time.)
func TestAuthentication(t *testing.T) {
	testAuthenticator := New()

	authData := []test{
		// Login: Bad Request
		{httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"\"}")),
			httptest.NewRecorder(),
			"No username in request body", 400},
		// Logout: 	Unauthorized
		{httptest.NewRequest(http.MethodDelete, "/auth", nil),
			httptest.NewRecorder(),
			"Missing or malformed bearer token", 401},
	}

	// Run preliminary tests.
	index := 0
	for _, d := range authData {
		testAuthenticator.ServeHTTP(d.w, d.r)
		res := d.w.Result()
		defer res.Body.Close()

		if res.StatusCode != d.code {
			t.Errorf("Test %d: Expected error code %d got %d", index, d.code, res.StatusCode)
		}
		index++
	}

	loginTest := test{httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"jingwu\"}")), httptest.NewRecorder(), "Successfully Logged in", 200}

	// Get login information out.
	testAuthenticator.ServeHTTP(loginTest.w, loginTest.r)
	res := loginTest.w.Result()
	defer res.Body.Close()
	if res.StatusCode != loginTest.code {
		t.Errorf("Test %d: Expected error code %d got %d", index, loginTest.code, res.StatusCode)
	}

	// For reading the body of response
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("Test %d: Expected no error, got %v", index, err)
	}

	// Unmarshal response
	var tokenMap map[string]string
	err = json.Unmarshal(data, &tokenMap)
	if err != nil {
		t.Errorf("Test %d: Expected no error, got %v", index, err)
	}

	token, ok := tokenMap["token"]
	if !ok {
		t.Errorf("Test %d: Expected response to have key token, got %v", index, tokenMap)
	}
	index++

	// Test that validate token works.
	validateTest1 := test{httptest.NewRequest(http.MethodGet, "/v1/db", http.NoBody), httptest.NewRecorder(), "", 0}
	validateTest1.r.Header.Set("Authorization", "Bearer "+token)

	// Should return correct username and valid bool
	valid, username := testAuthenticator.ValidateToken(validateTest1.w, validateTest1.r)
	if !valid {
		t.Errorf("Test %d: Expected to find valid user.", index)
	}
	if username != "jingwu" {
		t.Errorf("Test %d: Expected username jingwu, got %s", index, username)
	}
	index++

	// Test that validate token works.
	validateTest2 := test{httptest.NewRequest(http.MethodGet, "/v1/db", http.NoBody), httptest.NewRecorder(), "", 401}
	validateTest2.r.Header.Set("Authorization", "Bearer "+token+"z")

	// Should return invalid bool
	valid, username = testAuthenticator.ValidateToken(validateTest2.w, validateTest2.r)
	if valid {
		t.Errorf("Test %d: Expected to find invalid user.", index)
	}
	res = validateTest2.w.Result()
	defer res.Body.Close()
	if res.StatusCode != validateTest2.code {
		t.Errorf("Test %d: Expected error code %d got %d", index, validateTest2.code, res.StatusCode)
	}
	index++

	// Test logout works properly.
	logoutTest1 := test{httptest.NewRequest(http.MethodDelete, "/auth", http.NoBody), httptest.NewRecorder(), "Successfully Logged out", 204}
	logoutTest1.r.Header.Set("Authorization", "Bearer "+token)
	// Get logout information out.
	testAuthenticator.ServeHTTP(logoutTest1.w, logoutTest1.r)
	res = logoutTest1.w.Result()
	defer res.Body.Close()
	if res.StatusCode != logoutTest1.code {
		t.Errorf("Test %d: Expected error code %d got %d", index, logoutTest1.code, res.StatusCode)
	}
	index++

	// Test logout works properly.
	logoutTest2 := test{httptest.NewRequest(http.MethodDelete, "/auth", http.NoBody), httptest.NewRecorder(), "Successfully Logged out", 401}
	logoutTest2.r.Header.Set("Authorization", "Bearer "+token)
	// Get logout information out.
	testAuthenticator.ServeHTTP(logoutTest2.w, logoutTest2.r)
	res = logoutTest2.w.Result()
	defer res.Body.Close()
	if res.StatusCode != logoutTest2.code {
		t.Errorf("Test %d: Expected error code %d got %d", index, logoutTest2.code, res.StatusCode)
	}
	index++
}
