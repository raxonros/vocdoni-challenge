package node

import (
	"context"
	"go.vocdoni.io/dvote/ipfssync/subpub"
	libpeer "github.com/libp2p/go-libp2p-core/peer"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"math/big"
	"encoding/hex"
	"errors"

	"os"
    "os/signal"
    "syscall"
	"fmt"
	"time"
	"sync"
	"strings"
	"go.vocdoni.io/dvote/apiclient"
	"net/url"
	"github.com/google/uuid"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"encoding/gob"
    "bytes"
	"vocdoni-challenge/helpers"
	"vocdoni-challenge/pow"
	"strconv"


)

type VocdoniNode struct {
	Transport *subpub.SubPub
	PrivKey string
	GroupKey string
	Port int16
	NodeID string
	PubKeysNodes 	map[string]string
	NodePuzzleResolved map[string]int
	PublickKeyInterval   time.Duration
	Messages        chan *subpub.Message
	State 		int //0 to follower and 1 to Leader
	LeaderID	string
	NumNetworkNodes int
	VotingState int //0 to no voting and 1 to voting
	ApiClient *apiclient.HTTPclient
	ElectionID types.HexBytes
	ProofTree map[string]*apiclient.CensusProof
	PuzzlesResolved int
}

func StartNode(groupKey string, port int16) {

	log.Init("info", "stdout")

	VocNod := &VocdoniNode{
		PrivKey: util.RandomHex(32),
		GroupKey: groupKey,
		Port: port,
		PublickKeyInterval:   time.Second * 15,
		Messages: make(chan *subpub.Message),
		PubKeysNodes: 	make(map[string]string),
		NodePuzzleResolved: make(map[string]int),
		State: 0, //All nodes start as follower
		NumNetworkNodes: 1, //At first moment only exist one node
		VotingState: 0,
		PuzzlesResolved: 0,
	}

	//Create SubPub transport layer
	VocNod.Transport = subpub.NewSubPub(VocNod.PrivKey, []byte(VocNod.GroupKey), int32(VocNod.Port), false)
	VocNod.Transport.Start(context.Background(), VocNod.Messages)
	VocNod.NodeID = VocNod.Transport.NodeID
	VocNod.PubKeysNodes[VocNod.NodeID] = VocNod.Transport.PubKey

	hostURL, err := url.Parse("https://gw1.dev.vocdoni.net/v2")
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("connecting to %s", hostURL.String())

	//Created api client to vocdoni
	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	if err != nil {
		log.Fatal(err)
	}

	VocNod.ApiClient = api

	// Set the account in the API client, so we can sign transactions
	if err := api.SetAccount(VocNod.PrivKey); err != nil {
		log.Fatal(err)
	}

	log.Infof("Port %s\n", VocNod.Transport.Port)
	log.Infof("NodeID %s\n", VocNod.NodeID)
	log.Infof("Group Key  %s\n", VocNod.GroupKey)


	//Created to check peer added and removed
	peerAdded := make(chan libpeer.ID)
	VocNod.Transport.OnPeerAdd = func(id libpeer.ID) { peerAdded <- id }
	peerRemoved := make(chan libpeer.ID)
	VocNod.Transport.OnPeerRemove = func(id libpeer.ID) { peerRemoved <- id }

	go VocNod.sharePublickKey()
	go VocNod.handleEvents() // this spawns a single background task per VocdoniNode instance
	go VocNod.checkState(peerAdded, peerRemoved)

	//only uncomment to test, proof of work is called on the Voting Process
	//
	//go VocNod.proofOfWork()

	go func(){
		for startTime := time.Now(); time.Since(startTime) < time.Second*300; {
			if VocNod.NumNetworkNodes > 1 && VocNod.State == 1 {
				//This code init the election process but is not finished yet
				VocNod.InitElectionProcess()
				log.Info("Init de Election Process")
			}
			time.Sleep(50 * time.Second)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)

	go func() {
        sig := <-sigs
        fmt.Println()
        fmt.Println(sig)
        done <- true
    }()

    fmt.Println("awaiting signal")
    <-done
    fmt.Println("exiting")


}

//Function to do PoW for 20 seconds and count numbers of puzzles resolved
func (vc *VocdoniNode)proofOfWork(){

	for startTime := time.Now(); time.Since(startTime) < time.Second*20; {
		if ok :=pow.CalculatePow(); ok{
			vc.PuzzlesResolved += 1
		}
	}
	vc.NodePuzzleResolved[vc.NodeID]=vc.PuzzlesResolved
	log.Infof("Puzzles Resolved by %s: %s\n",vc.NodeID, vc.PuzzlesResolved )
	if vc.VotingState == 1 {
		vc.Transport.SendBroadcast(subpub.Message{Data: []byte("Pow:"+vc.NodeID+":"+strconv.Itoa(vc.PuzzlesResolved))})
	}
	vc.PuzzlesResolved = 0

}

//This function init the election process
func (vc *VocdoniNode) InitElectionProcess(){


	//This code is not finished yet
	if vc.State == 1 {

		acc, err := vc.ApiClient.Account("")
		if err != nil {
			var faucetPkg []byte

			// Get the faucet package of bootstrap tokens
			log.Infof("getting faucet package")
			faucetPkg, err = helpers.GetFaucetPkg(vc.ApiClient.MyAddress().Hex())
			if err != nil {
				log.Fatal(err)
			}

			// Create the organization account and bootstraping with the faucet package
			log.Infof("creating Vocdoni account %s", vc.ApiClient.MyAddress().Hex())
			log.Debugf("faucetPackage is %x", faucetPkg)
			hash, err := vc.ApiClient.AccountBootstrap(faucetPkg)
			if err != nil {
				log.Fatal(err)
			}
			helpers.EnsureTxIsMined(vc.ApiClient, hash)
			acc, err = vc.ApiClient.Account("")
			if err != nil {
				log.Fatal(err)
			}
			if acc.Balance == 0 {
				log.Fatal("account balance is 0")
			}
		}

		log.Infof("account %s balance is %d", vc.ApiClient.MyAddress().Hex(), acc.Balance)

		var slice_pub_keys []string
		for _ , v := range vc.PubKeysNodes{
			slice_pub_keys = append(slice_pub_keys, v)
		}
		// Create a new census
		censusID, err := vc.ApiClient.NewCensus("weighted")
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("new census created with id %s", censusID.String())

		// Add the accounts to the census
		participants := &vapi.CensusParticipants{}
		for i, voterAccount := range slice_pub_keys {
			bytes_pub_key,  _ := hex.DecodeString(voterAccount)
			participants.Participants = append(participants.Participants,
				vapi.CensusParticipant{
					Key:    bytes_pub_key,
					Weight: (*types.BigInt)(new(big.Int).SetUint64(10)),
				})

			if i == len(slice_pub_keys)-1 || ((i+1)%vapi.MaxCensusAddBatchSize == 0) {
				if err := vc.ApiClient.CensusAddParticipants(censusID, participants); err != nil {
					log.Fatal(err)
				}
				log.Infof("added %d participants to census %s", len(participants.Participants), censusID.String())
				participants = &vapi.CensusParticipants{}
			}

		}

		size, err := vc.ApiClient.CensusSize(censusID)
		if err != nil {
			log.Fatal(err)
		}
		if size != uint64(vc.NumNetworkNodes) {
			log.Fatalf("census size is %d, expected %d", size, vc.NumNetworkNodes)
		}
		log.Infof("census %s size is %d", censusID.String(), size)

		// Publish the census
		root, censusURI, err := vc.ApiClient.CensusPublish(censusID)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("census published with root %s", root.String())

		// Check census size (of the published census)
		size, err = vc.ApiClient.CensusSize(root)
		if err != nil {
			log.Fatal(err)
		}
		if size != uint64(vc.NumNetworkNodes) {
			log.Fatalf("published census size is %d, expected %d", size, len(vc.PubKeysNodes))
		}

		// Generate the voting proofs
		type voterProof struct {
			proof   *apiclient.CensusProof
			address string
		}
		proofs := make(map[string]*apiclient.CensusProof, vc.NumNetworkNodes)

		addNaccounts := func(accounts []string, wg *sync.WaitGroup) {
			defer wg.Done()
			log.Infof("generating %d voting proofs", len(accounts))
			for _, acc := range accounts {
				bytes_pub_key, _ := hex.DecodeString(acc)
				pr, err := vc.ApiClient.CensusGenProof(root, bytes_pub_key)
				if err != nil {
					log.Fatal(err)
				}
				proofs[acc] = pr
			}
		}


		var wg sync.WaitGroup
		wg.Add(1)
		go addNaccounts(slice_pub_keys, &wg)

		wg.Wait()
		time.Sleep(time.Second) // wait a grace time for the last proof to be added
		log.Debugf("%d/%d voting proofs generated successfully", len(proofs), len(vc.PubKeysNodes))


		// Create a new Election
		var choice_metadata []vapi.ChoiceMetadata

		//We create a Choice metada with pub keys
		for k, v := range slice_pub_keys{
			choice := vapi.ChoiceMetadata{
				Title:  map[string]string{v: "Yes"},
				Value: uint32(k),
			}
			choice_metadata = append(choice_metadata, choice)
		}
		electionID, err := vc.ApiClient.NewElection(&vapi.ElectionDescription{
			Title:       map[string]string{"default": fmt.Sprintf("Vocdoni election %s", util.RandomHex(8))},
			Description: map[string]string{"default": "Vocdoni election to remove worst node"},
			EndDate:     time.Now().Add(time.Minute * 20),

			VoteType: vapi.VoteType{
				UniqueChoices:     false,
				MaxVoteOverwrites: 1,
			},

			ElectionType: vapi.ElectionType{
				Autostart:         true,
				Interruptible:     true,
				Anonymous:         false,
				SecretUntilTheEnd: false,
				DynamicCensus:     false,
			},

			Census: vapi.CensusTypeDescription{
				RootHash: root,
				URL:      censusURI,
				Type:     "weighted",
			},

			Questions: []vapi.Question{
				{
					Title:       map[string]string{"default": "Worst Node"},
					Description: map[string]string{"default": "Group of nodes to remove of our network"},
					Choices: choice_metadata,
				},
			},
		})
		if err != nil {
			log.Fatal(err)
		}

		election := helpers.EnsureElectionCreated(vc.ApiClient, electionID)
		log.Infof("created new election with id %s", electionID.String())
		log.Debugf("election details: %+v", *election)

		// Wait for the election to start
		helpers.WaitUntilElectionStarts(vc.ApiClient, electionID)

		vc.ElectionID = electionID
		vc.ProofTree = proofs
		vc.Transport.SendBroadcast(subpub.Message{Data: []byte("electionID:"+electionID.String())})


		var bufProofs bytes.Buffer
		gob.NewEncoder(&bufProofs).Encode(proofs)

		vc.Transport.SendBroadcast(subpub.Message{Data:bufProofs.Bytes()})

		time.Sleep(5 * time.Second)
		vc.Transport.SendBroadcast(subpub.Message{Data: []byte("The election Start")})

		vc.InitVoting()

	}
}

//This Function init de Voting Process
func (vc *VocdoniNode) InitVoting(){

	vc.VotingState = 1
	vc.proofOfWork()
	startTime := time.Now()
	log.Info("Voting...")

	contextDeadlines := 0

	c := vc.ApiClient.Clone(vc.PrivKey)
	_, err := c.Vote(&apiclient.VoteData{
		ElectionID: vc.ElectionID,
		ProofTree:  vc.ProofTree[vc.Transport.PubKey],
		Choices:    []int{1},
		KeyType:    models.ProofArbo_ADDRESS,
	})

	// if the context deadline is reached, we don't need to print it (let's jus retry)
	if err != nil && errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		contextDeadlines++
	} else if err != nil && !strings.Contains(err.Error(), "already exists") {
		// if the error is not "vote already exists", we need to print it
		log.Warn(err)

	}

	log.Infof("Vote sent")

	// Wait for all the votes to be verified
	log.Infof("waiting for all the votes to be registered...")
	for {
		count, err := vc.ApiClient.ElectionVoteCount(vc.ElectionID)
		if err != nil {
			log.Warn(err)
		}
		if count == uint32(vc.NumNetworkNodes) {
			break
		}
		time.Sleep(time.Second * 5)
		log.Infof("verified %d/%d votes", count, vc.NumNetworkNodes)
		if time.Since(startTime) > time.Duration(5)*time.Minute {
			log.Fatalf("timeout waiting for votes to be registered")
		}
	}
	log.Infof("%d votes registered successfully, took %s. At %d votes/second",
		vc.NumNetworkNodes, time.Since(startTime), int(float64(vc.NumNetworkNodes)/time.Since(startTime).Seconds()))

	// End the election by seting the status to ENDED
	log.Infof("ending election...")
	hash, err := vc.ApiClient.SetElectionStatus(vc.ElectionID, "ENDED")
	if err != nil {
		log.Fatal(err)
	}

	// Check the election status is actually ENDED
	helpers.EnsureTxIsMined(vc.ApiClient, hash)
	election, err := vc.ApiClient.Election(vc.ElectionID)
	if err != nil {
		log.Fatal(err)
	}
	if election.Status != "ENDED" {
		log.Fatal("election status is not ENDED")
	}
	log.Infof("election %s status is ENDED", vc.ElectionID.String())

	// Wait for the election to be in RESULTS state
	log.Infof("waiting for election to be in RESULTS state...")
	helpers.WaitUntilElectionStatus(vc.ApiClient, vc.ElectionID, "RESULTS")
	log.Infof("election %s status is RESULTS", vc.ElectionID.String())

	election, err = vc.ApiClient.Election(vc.ElectionID)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("election results: %v", election.Results)
	vc.VotingState = 0
}


//This function detect when one peer is added and if you are the new Node ask por the leader
func (vc *VocdoniNode) checkState(peerAdded chan libpeer.ID, peerRemoved chan libpeer.ID) {

	for {
		select {

		case nodeAddedID:= <-peerAdded:

			log.Infof("Peer added %s\n", nodeAddedID)
			vc.NumNetworkNodes += 1
			if vc.LeaderID == "" {
				go vc.whoIsLeader()
			}

		case nodeRemovedID:= <-peerRemoved:
			log.Infof("Peer removed %s\n", nodeRemovedID)
			delete(vc.PubKeysNodes, nodeRemovedID.String())
			vc.NumNetworkNodes -= 1

			if vc.LeaderID == nodeRemovedID.String() {
				log.Info("Choose leader again")
				vc.chooseLeader()
			}
		}
	}
}

//This function choose the leader using the most value of hex pub key
func (vc *VocdoniNode) chooseLeader() {
	max_value := "0"
	var node_id_max string
	for node_id, pub_key := range vc.PubKeysNodes{
		if  pub_key > max_value {
			max_value = pub_key
			node_id_max = node_id
		}

	}
	vc.LeaderID = node_id_max
	if vc.NodeID == vc.LeaderID {
		vc.State = 1
	}

	log.Infof("Leader choose is %s\n ", vc.LeaderID)
}

//This function as for the leader 5 times if dont find choose a Leader
func (vc *VocdoniNode) whoIsLeader() {
	askLeaderTicker := time.NewTicker(10 * time.Second)
	defer askLeaderTicker.Stop()

	counter := 0
	for {
		select {

		case <-askLeaderTicker.C:
			// ask for the leader
			if counter < 5 {
				vc.Transport.SendBroadcast(subpub.Message{Data: []byte("who is leader")})
				counter = counter + 1
			} else {
				if vc.LeaderID == "" {
					vc.chooseLeader()
				}
				return
			}

		}
	}
}

//This function manage de differents messages
func (vc *VocdoniNode) handleEvents() {
	for {
		select {
		case d := <-vc.Messages:
			//If one node ask for the leader, share the leader info
			if string(d.Data) == "who is leader"{
				if vc.LeaderID != "" {
					vc.Transport.SendBroadcast(subpub.Message{Data: []byte("the leader is "+":"+vc.LeaderID)})

				}

			//If we receive the infor leader we set the laeder info in our struct
			} else if strings.Contains(string(d.Data), "the leader is") {
				res := strings.Split(string(d.Data), ":")
				vc.LeaderID = res[1]
				if vc.LeaderID == vc.NodeID {
					vc.State = 1
				}

			//If we receive the start meesage election we init de election
			} else if strings.Contains(string(d.Data), "election Start"){
				go vc.InitVoting()
				log.Info("The election starts")

			//If we receive the pow we set the pow data in our struct
			} else if strings.Contains(string(d.Data), "Pow"){
				res := strings.Split(string(d.Data), ":")
				numResolvePuzles, _ := strconv.Atoi(res[2])
				vc.NodePuzzleResolved[res[1]] = numResolvePuzles

			//If we receive the electionID we set the electionID in our struct
			}else if strings.Contains(string(d.Data), "electionID"){
				res := strings.Split(string(d.Data), ":")
				bytes_pub_key, _ := hex.DecodeString(res[1])
				vc.ElectionID = bytes_pub_key

			//We pass de node id and publick key in our struct map
			}else if strings.Contains(string(d.Data), "PubKey"){
				res:= strings.Split(string(d.Data), ":")
				_, ok := vc.PubKeysNodes[res[1]]

				if !ok{
					vc.PubKeysNodes[res[1]] = res[2]
				}

			//We receive census proof and add in our struct
			}else {

				 var outputBuffer bytes.Buffer
				 outputBuffer.Write(d.Data)
				 var proofTree map[string]*apiclient.CensusProof

				 gob.NewDecoder(&outputBuffer).Decode(&proofTree)

				 vc.ProofTree = proofTree
			}
		}
	}
}

//This funtion share periodically the publick keys
func (vc *VocdoniNode) sharePublickKey() {
	sahrePublicKeyTicker := time.NewTicker(vc.PublickKeyInterval)
	defer sahrePublicKeyTicker.Stop()

	for {
		select {

		case <-sahrePublicKeyTicker.C:
			// send NodeId and PublickKey
			vc.Transport.SendBroadcast(subpub.Message{Data: []byte("PubKey:"+vc.NodeID+":"+vc.Transport.PubKey)})
		}
	}
}

