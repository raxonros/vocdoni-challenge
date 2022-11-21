package helpers

import (
	"time"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"net/http"
	"net/url"
	"encoding/json"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
	"io/ioutil"
)

const (
	faucetURL   = "https://faucet-azeno.vocdoni.net/faucet/vocdoni/dev/"
	faucetToken = "158a58ba-bd3e-479e-b230-2814a34fae8f"
)

func EnsureElectionCreated(api *apiclient.HTTPclient, electionID types.HexBytes) *api.Election {
	for startTime := time.Now(); time.Since(startTime) < time.Second*40; {
		election, _ := api.Election(electionID)
		if election != nil {
			return election
		}
		time.Sleep(time.Second * 4)
	}
	log.Fatalf("election %s not created", electionID.String())
	return nil
}

func WaitUntilElectionStarts(api *apiclient.HTTPclient, electionID types.HexBytes) {
	election, err := api.Election(electionID)
	if err != nil {
		log.Fatal(err)
	}
	startHeight, err := api.DateToHeight(election.StartDate)
	if err != nil {
		log.Fatal(err)
	}
	waitUntilHeight(api, startHeight+1) // add a block to be sure
}

func waitUntilHeight(api *apiclient.HTTPclient, height uint32) {
	for {
		info, err := api.ChainInfo()
		if err != nil {
			log.Warn(err)
		} else {
			if *info.Height > height {
				break
			}
		}
		time.Sleep(time.Second * 4)
	}
}

func EnsureTxIsMined(api *apiclient.HTTPclient, txHash types.HexBytes) {
	for startTime := time.Now(); time.Since(startTime) < 40*time.Second; {
		_, err := api.TransactionReference(txHash)
		if err == nil {
			return
		}
		time.Sleep(4 * time.Second)
	}
	log.Fatalf("tx %s not mined", txHash.String())
}

func WaitUntilElectionStatus(api *apiclient.HTTPclient, electionID types.HexBytes, status string) {
	for startTime := time.Now(); time.Since(startTime) < time.Second*300; {
		election, err := api.Election(electionID)
		if err != nil {
			log.Fatal(err)
		}
		if election.Status == status {
			return
		}
		time.Sleep(time.Second * 5)
	}
	log.Fatalf("election status %s not reached", status)
}

func GetFaucetPkg(accountAddress string) ([]byte, error) {
	var c http.Client
	u, err := url.Parse(faucetURL + accountAddress)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(&http.Request{
		Method: "GET",
		URL:    u,
		Header: http.Header{
			"Authorization": []string{"Bearer " + faucetToken},
			"User-Agent":    []string{"Vocdoni API client / 1.0"},
		},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Debugf("received response from faucet: %s", data)
	type faucetResp struct {
		Amount    uint32         `json:"amount"`
		Payload   []byte         `json:"faucetPayload"`
		Signature types.HexBytes `json:"signature"`
	}

	var fResp faucetResp
	json.Unmarshal(data, &fResp)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(&models.FaucetPackage{
		Payload:   fResp.Payload,
		Signature: fResp.Signature})
}