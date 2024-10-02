// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	oos "github.com/okteto/remote/pkg/os"
	ossh "github.com/okteto/remote/pkg/ssh"
)

const (
	localhost = "127.0.0.1"
	sshPort   = 12201
)

var (
	sshServer   *ossh.Server
	stopChan    chan struct{} // Channel to signal stopping the server goroutine
	brokenBytes = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAh2Y0gWRHG/putBKgQUJg7Al6ocWFegjv7yKS0CWdcpyDaBJC
hOCCxrQMENpm6DXg1KQ45cIps3qaRcB+HS13gg8aNcxctJhZ0Z2raDucSwuK3x+U
j3KeJQThGfH7q4ufmj0hO37c6JnQOUxdsdc9s5t3k9PGx6mbNYSS6Ru1P7KfmK3a
EZYrgJdyiLYrGJM9pBMhC8OoxVDCAYdcYDhQO6fvYeztmS/XbZPFAo+Upi9v+DvY
eeRaJYIg0vPbeWqtBtTsBYFPZUV8ALY+GmZxG6FLNRDzFfGPbsfHb6jVVgVTNoG9
YdS7z+YQWPxOedcwamNcBvthwqJ0qLKcW4d6AwIDAQABAoIBABoFb2PKlMvwtiPP
TJMeIahbLgE3B67rVsk7eMGd6QNkxvBSSgxlZVywr3zxrENGT34uTW0Cgqcmkc2D
7/jFiykoo93g95QRj3l0dsOiaSgXAMpetFtZKMFujaBB3B8cx0CKLP/VIrllDhpk
7cDG3+nLoBB9677lfpmrb9xy4cNq1DcIsqnYmw9SGIL9fB1uEe5KbC/6/MnPMR6C
qsO0h4YREHv3XkVwsnkC46uDjcRpFg+WFrxzijAIxwhjjrwBRqsr7ls6x6uIpWIS
iBdE2pTRIpFWG4hdHZpQDtNF7mTFmHsL6J74aQ/z/k/nuZW+5wET5GGHbc2uCapc
5yhWE5ECgYEA6t1Aw+e2b0LTcY6dCDfa8Necq5yTWRjmafRtkwednAQL0Rw3Zdlm
jeavJHaMbk+90SEdyC7ZeUo5o/EKfbLI9QHmBv/72Gx8x4UVn3bUw3lKcQpy5ujj
Hpds1SujxdGLgOUa4sO4+ZRgcbZI5yozDGHbbCaBXeN9Hx/QvXX51nkCgYEAk5WA
Hg8j8x8m+OK5Uc+pvVVzGGVtzPbT2u7r8mVnRpbssqCwbHnoYxUAdZIQlEd3ho7W
Ogfhn/dAdTdjhSz4dFiz62aIRY3YUROVMYPnK6OGHx8UPj+e6lF1JKVdGOTCEtvZ
8aDpp9ezwNM8xhmLiBimuIrO8uCXmOKmCsZO5VsCgYEAlLkede+Y2sOaWWJRlg6o
mbIvB4zS0ayu25FogZ5k85KsWPJhMiGEi370kUZwjrn4HSG3Vjg0hlW2QlY0qnw4
PM3C99PJirbIHR/tHVmGSXHY7dQMBqSajZid1i8YOgADMs/hQo9l5sVKfTdM2vUh
9klRRR7s9KNFbBwddpvqZSECgYBvyixzmHBYtl4k5EgPykb+nX1DG4FiYgzzp0qe
H2VtOOEuJT29zVHTy5K/e9aHGuNUz1lCN7oWG4l9wGjn9bp1FsOiKdZLKTiToUu/
Z/RxP7pWVIpW5IYRY4ZiLcgpFQ5Ujqe1uXeDpYYJJ3i/bZYdjt6qpqo/mxP5wKSZ
QMF+RQKBgQCnCCtCk8QjOPuBd94kL9jihvle2sMQIARM0BOTecBgjdnWR4M83KF5
iqh8pe9a/izPSyhJT5XvXN8lsQy0/h9racsRqbYLJ4gi21U8a3Y8ZTN19isyA1KN
v3sTyoEAYm8aQ/I9pjRtO+msQenFo2sJmR19T1qKtq4OyoPjkGH7og==
-----END RSA PRIVATE KEY-----`)
	privateBytes = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAiA6fVUmkZqFeTB6yiZgWfIR2ych9WDtOROjr0H2sSoat23IK
Z7koaDcwDiMtYgKloEzL83yRUCiO3nQswd5rD7oX/mNM457hhLykv68PeLa4yxFe
1ksKnRqw3QFiWlS2ahQB60VRRYIFwCONnxb4AY8tJE+jEtoe5FgTTa1QqHhtDaNu
swu1suFdbccLwgL5dMaRG5922UPthf/MIoprBg9gwImgVsxiHAQ/BAChzGZZTb8f
QBTtsqWuHH1t3lBlgDVCJnD4Y/MMHUMnKknKKkOIcE9E/8cod+S+u2RVbpaK37cz
zT0Ri8Ehtqh87h0wAvjnRB2ViIi4CiiA8IjvFQIDAQABAoIBABSGECbvautIYE8f
OlNjK9EwsjKSGcZbe90NPbU/n+PGGSfHNiabWasO2CLpz4i/WXiq3QEnw0vSMmsA
H1hHUuMWcVQgapLTfrVlN3zqzWyqM4n+Xs34l+tHAXL1KH7z1GiACCITXMUdbfyj
4FMlRdgGXwrhMHpBiPNtDcxj3ozrE/gAEZZIiqp8BKhOzmEs24sEnbw3R38LTLsf
4m6kEmi4ypOgUMyKiwM2y8tqdKFByg2sgUcOPmKD1VZF6SOxG6d8c74BqagQEXL4
8K8zPwOKqvukpkX586B+e1DUaTMUxSAc+Y4WtWEUl3+FgJLn+oweUorxSh8H8uXB
jU/0IsECgYEA56koTA0yDkQWk6kEP64qfbYwPg6IwtFtM22WPD0Br3bnv4oHNzuN
7TGNvD8WOsig9WTCbwgAjVI0H/rosaxopFRhBgrewM5+rw1SjSSl8DwNI5pzpvSk
FEgsYvlGjo5P2GZg2yC/WyuKzCwkKshlPAaRQkKnIP2ocBuyl6ZuRF0CgYEAlloR
9a9F0dKTFXpM2kTBawT3XDJMumR4oSxV8ewZM2G0Gn7JJS/fo9giU0Wbu0kBBNY5
dzWN5CnnEkwsFPlYAFcSP3ZSyFhZV3Ej71E0PMcrtiNly9yfl+C8qXYy2tQbMBg6
SNmlPgU3nSoOpo4dHSglLipu6wfYNBp4hmHxKhkCgYBZiH4jjoIHE3eqUAttVTHk
7jIM+e8PZIOQ+cyzsVxiZVFCLOWHCkRbJOjDlZpQ09Opo+ylnvEfrWKp7X5blOfI
gzgQbskSuXwqybpbBIwBoiPt27bREwILOK22/uKvG/9uAntylWB/qv9006aSxmqh
3WPsuPgA/yhHU90dUVySnQKBgHrHRs9IuE1mFjJ90vCTIRfR0GX7tWioz1FesTiD
wviQmtDgJEY31/smbrFtsIS6UxiuD/NnSl2+UjJA/eaKz/BHKmGksGlmrF6Zx+yQ
dYYEZ4+vTFuz6KfQIICKQ3ErsKAPyNjg0u3YuNehFqGwf9nlhKc0g6tuloHF9BJZ
POVRAoGBAOV6Y297tKhhEtI3nhzuWnvEzlg3fnI8mQMSkHkPwbItdICOG/aatvRw
oDzRvq1EfAspjBVpXoNFCL085eXwxBp45bTLeg/tHddpQDR9J8EI0KeohmlBtNcv
DQ+6C4ljC/2NLkRqdLHTQ6vpKDFJ9bQ5hS2SyBE4SQZw7PqnDJR+
-----END RSA PRIVATE KEY-----`)
)

func TestHost(t *testing.T) {
	t.Run("hostRunTest", hostRunTest)
}

func hostRunTest(t *testing.T) {
	// prepare ssh keys
	privKey, err := os.CreateTemp("", "unit-test-ssh-private-key")
	require.NoError(t, err)
	defer os.Remove(privKey.Name())
	_, err = privKey.Write(privateBytes)
	require.NoError(t, err)
	require.NoError(t, privKey.Close())

	block, _ := pem.Decode(privateBytes)
	require.NotNil(t, block)
	// Parse the private key
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	publicKey := privateKey.PublicKey
	publicKeyPEM := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&publicKey),
	}
	pubKey, err := os.CreateTemp("", "unit-test-ssh-public-key")
	require.NoError(t, err)
	defer os.Remove(pubKey.Name())
	err = pem.Encode(pubKey, publicKeyPEM)
	require.NoError(t, err)
	require.NoError(t, pubKey.Close())

	publicKeyBytes, err := os.ReadFile(pubKey.Name())
	require.NoError(t, err)
	// Decode the PEM encoded public key
	block, _ = pem.Decode(publicKeyBytes)
	require.NotNil(t, block)
	// Parse the public key
	parsedKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	require.NoError(t, err)
	authKey, err := os.CreateTemp("", "unit-test-ssh-auth-key")
	require.NoError(t, err)
	sshPublicKey, err := ssh.NewPublicKey(parsedKey)
	require.NoError(t, err)
	_, err = authKey.Write(ssh.MarshalAuthorizedKey(sshPublicKey))
	require.NoError(t, err)
	require.NoError(t, authKey.Close())

	brokenKey, err := os.CreateTemp("", "unit-test-ssh-broken-key")
	require.NoError(t, err)
	defer os.Remove(brokenKey.Name())
	_, err = brokenKey.Write(brokenBytes)
	require.NoError(t, err)
	require.NoError(t, brokenKey.Close())

	err = startSSHServer(sshPort, authKey.Name(), t)
	require.NoError(t, err)
	defer stopSSHServer()

	host := &Host{
		NodeID:            constants.E2EDocker + "_unittest",
		IP:                localhost,
		SSHPrivateKeyPath: privKey.Name(),
		SSHUser:           constants.AnsibleSSHUser,
		SSHCommonArgs:     constants.AnsibleSSHUseAgentParams,
	}
	brokenHost := &Host{
		NodeID:            constants.E2EDocker + "_broken",
		IP:                localhost,
		SSHPrivateKeyPath: brokenKey.Name(),
		SSHUser:           constants.AnsibleSSHUser,
		SSHCommonArgs:     constants.AnsibleSSHUseAgentParams,
	}

	// good connection
	require.NoError(t, host.WaitForPort(sshPort, 10*time.Second))
	require.NoError(t, host.Connect(sshPort))
	require.True(t, host.Connected())
	require.NoError(t, host.MkdirAll("/tmp/test", time.Second))
	require.DirExists(t, "/tmp/test")
	require.NoError(t, host.StreamSSHCommand("sleep 1 && ls /tmp/test", nil, 10*time.Second))

	// test  upload
	randomString := utils.RandomString(20)
	remoteFile := "/tmp/test/upload-unittest"
	tmpFile, err := os.CreateTemp("", "upload-unittest")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.Write([]byte(randomString))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	require.NoError(t, host.Upload(tmpFile.Name(), remoteFile, 1*time.Second))

	require.FileExists(t, remoteFile)
	content, err := os.ReadFile(remoteFile)
	require.NoError(t, err)
	require.Equal(t, randomString, string(content))

	// test download
	localFile := "/tmp/download-unittest"
	require.NoError(t, host.Download(remoteFile, localFile, 1*time.Second))
	require.FileExists(t, localFile)
	content, err = os.ReadFile(remoteFile)
	require.NoError(t, err)
	require.Equal(t, randomString, string(content))

	_, err = host.Command("touch /tmp/test/streamtest", nil, 10*time.Second)
	require.NoError(t, err)
	require.FileExists(t, "/tmp/test/streamtest")

	// bad connection
	require.Error(t, brokenHost.Connect(sshPort))
	require.Error(t, brokenHost.MkdirAll("/tmp/test", time.Second))
}

func startSSHServer(sshPort int, keyFileName string, t *testing.T) error {
	stopChan = make(chan struct{})
	// Get shell
	shell, err := oos.GetShell()
	if err != nil {
		return err
	}
	t.Logf("Using shell: %s\n", shell)
	// Load authorized keys
	keys, err := ossh.LoadAuthorizedKeys(keyFileName)
	t.Logf("Loading authorized keys from %s", keyFileName)
	if err != nil {
		return err
	}
	// Create SSH server configuration
	srv := ossh.Server{
		Port:           sshPort,
		Shell:          shell,
		AuthorizedKeys: keys,
	}

	// Start SSH server in a separate goroutine
	go func() {
		t.Logf("SSH server %s started on 0.0.0.0:%d\n", "unitest", srv.Port)
		defer t.Log("SSH server stopped")
		for {
			select {
			case <-stopChan:
				t.Log("Received stop signal.")
				return // Stop the goroutine
			default:
				// Do whatever the goroutine is supposed to do
				err := srv.ListenAndServe()
				if err != nil {
					t.Logf("SSH server error: %v", err)
					return
				}
			}
		}
	}()

	// Assign the server to the global variable
	sshServer = &srv
	return nil
}

func stopSSHServer() {
	if sshServer != nil {
		close(stopChan) // Close the stop channel to signal stopping the server goroutine
	}
}

func TestExpandHome(t *testing.T) {
	// Mock Host with SSHUser set to "testuser"
	host := &Host{SSHUser: "testuser"}

	// Test case 1: path starts with "~/"
	input1 := "~/documents/file.txt"
	expected1 := filepath.Join("/home", "testuser", "documents/file.txt")
	result1 := host.ExpandHome(input1)
	require.Equal(t, expected1, result1)

	// Test case 2: path starts with "~" but with no following directory
	input2 := "~"
	expected2 := filepath.Join("/home", "testuser")
	result2 := host.ExpandHome(input2)
	require.Equal(t, expected2, result2)

	// Test case 3: path does not start with "~/"
	input3 := "/var/www"
	expected3 := "/var/www"
	result3 := host.ExpandHome(input3)
	require.Equal(t, expected3, result3)

	// Test case 4: empty input
	input4 := ""
	expected4 := "/home/testuser"
	result4 := host.ExpandHome(input4)
	require.Equal(t, expected4, result4)

	// Test case 5: path starts with "~/" but with no following directory
	input5 := "~/"
	expected5 := filepath.Join("/home", "testuser", "/")
	result5 := host.ExpandHome(input5)
	require.Equal(t, expected5, result5)
}
