package handlers

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	dem "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/events"
)

type errorResponse struct {
	Message    string `json:"message"`
	Code       string `json:"code"`
	HTTPStatus int    `json:"http_status"`
}

// Match ...
type Match struct {
	Errors  []errorResponse `json:"errors"`
	ID      string          `json:"match_id"`
	DemoURL []string        `json:"demo_url"`
}

// PlayerHistory ...
type PlayerHistory struct {
	Errors          []errorResponse `json:"errors"`
	MatchID         string          `json:"match_id"`
	CompetitionType string          `json:"competition_type"`
}

// Player ...
type Player struct {
	Errors   []errorResponse `json:"errors"`
	PlayerID string          `json:"player_id"`
	Nickname string          `json:"nickname"`
	History  []PlayerHistory `json:"items"`
}

// CrosshairResponse ...
type CrosshairResponse struct {
	Code string `json:"code"`
}

func requestFaceIt(endpoint string) ([]byte, error) {
	client := &http.Client{}

	bearer := "Bearer " + os.Getenv("FACEIT_API_KEY")

	req, err := http.NewRequest("GET", "https://open.faceit.com/data/v4"+endpoint, nil)
	req.Header.Set("Authorization", bearer)
	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
		return nil, err
	}

	return ioutil.ReadAll(resp.Body)
}

func getPlayer(steamID string) (Player, error) {
	playerResponse, err := requestFaceIt("/players?game=csgo&game_player_id=" + steamID)
	if err != nil {
		return Player{}, err
	}

	var player Player
	json.Unmarshal(playerResponse, &player)
	if len(player.Errors) > 0 {
		return player, errors.New(player.Errors[0].Message)
	}

	playerHistoryResponse, err := requestFaceIt("/players/" + player.PlayerID + "/history?game=csgo&offset=0&limit=20")
	if err != nil {
		return Player{}, err
	}
	json.Unmarshal(playerHistoryResponse, &player)
	if len(player.Errors) > 0 {
		return player, errors.New(player.Errors[0].Message)
	}

	return player, nil
}

func getMatchInfo(matchID string) (Match, error) {
	matchResponse, err := requestFaceIt("/matches/" + matchID)
	if err != nil {
		return Match{}, err
	}

	var match Match
	json.Unmarshal(matchResponse, &match)
	if len(match.Errors) > 0 {
		return match, errors.New(match.Errors[0].Message)
	}

	return match, nil
}

func downloadDemo(fileName string, url string) (*os.File, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Create the file
	out, err := ioutil.TempFile(os.TempDir(), fileName+"_*.dem")
	if err != nil {
		return out, err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, reader)
	return out, err
}

// CrosshairHandler ...
func CrosshairHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	playerID := mux.Vars(req)["playerID"]

	player, err := getPlayer(playerID)
	if err != nil {
		http.Error(w, "Could not find user with steam id: "+playerID, 500)
		return
	} else if len(player.History) == 0 {
		http.Error(w, "User "+playerID+" has not played for at least 6 months", 500)
		return
	}

	var latestMatchID string
	for _, history := range player.History {
		if history.CompetitionType != "championship" {
			latestMatchID = history.MatchID
			break
		}
	}

	latestMatch, err := getMatchInfo(latestMatchID)
	if err != nil {
		http.Error(w, "Error getting user "+playerID+" latest match", 500)
		return
	}

	demoFile, err := downloadDemo(latestMatch.ID, latestMatch.DemoURL[0])
	if err != nil {
		panic(err)
	}

	f, err := os.Open(demoFile.Name())
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var playerCrosshair string
	p := dem.NewParser(f)

	p.RegisterEventHandler(func(start events.MatchStart) {
		for _, pl := range p.GameState().Participants().Playing() {
			if fmt.Sprint(pl.SteamID64) == playerID {
				v, b := pl.ResourceEntity().PropertyValue(fmt.Sprintf("m_szCrosshairCodes.%03d", pl.Entity.ID()))
				if b == true && v.StringVal != "" {
					playerCrosshair = v.StringVal
				}
			}
		}
	})
	err = p.ParseToEnd()
	if err != nil {
		panic(err)
	}
	defer p.Close()
	defer os.Remove(demoFile.Name())

	fmt.Fprintf(w, "%s\n", playerCrosshair)
}
