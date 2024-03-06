// TODO: cleanup api calls into a util func and handle 429s / errors with retries
// or handle them gracefully
package main

import (
	"cmp"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anthonyposchen/auosrs.git/templates"
	"github.com/anthonyposchen/auosrs.git/templates/route"
	"github.com/mergestat/timediff"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
	"golang.org/x/time/rate"
)

const (
	clanID = 276

	RankOwner = iota + 1
	RankDeputyOwner
	RankMaster
	RankGeneral
	RankMajor
	RankProselyte
	RankZenyte
	RankOnyx
	RankDragonStone
	RankDiamond
	RankRuby
	RankEmerald
	RankSapphire
	RankMember
	RankGuest
)

var Bosses = []string{"abyssal_sire", "alchemical_hydra", "artio", "barrows_chests", "bryophyta", "callisto", "calvarion", "cerberus", "chambers_of_xeric", "chambers_of_xeric_challenge_mode", "chaos_elemental", "chaos_fanatic", "commander_zilyana", "corporeal_beast", "crazy_archaeologist", "dagannoth_prime", "dagannoth_rex", "dagannoth_supreme", "deranged_archaeologist", "duke_sucellus", "general_graardor", "giant_mole", "grotesque_guardians", "hespori", "kalphite_queen", "king_black_dragon", "kraken", "kreearra", "kril_tsutsaroth", "mimic", "nex", "nightmare", "phosanis_nightmare", "obor", "phantom_muspah", "sarachnis", "scorpia", "scurrius", "skotizo", "spindel", "tempoross", "the_gauntlet", "the_corrupted_gauntlet", "the_leviathan", "the_whisperer", "theatre_of_blood", "theatre_of_blood_hard_mode", "thermonuclear_smoke_devil", "tombs_of_amascut", "tombs_of_amascut_expert", "tzkal_zuk", "tztok_jad", "vardorvis", "venenatis", "vetion", "vorkath", "wintertodt", "zalcano", "zulrah"}

type ClanInfo struct {
	// map of boss name to the amount of kills in 1 week
	Bosses map[string]int
	Info   route.Info
	sync.Mutex
}

var clanInfo = ClanInfo{Mutex: sync.Mutex{}, Bosses: make(map[string]int)}

//go:embed dist
var dist embed.FS

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		log.Fatal(err)
	}
	return http.FS(fsys)
}

func main() {
	go wiseOldmanSync(os.Getenv("TOKEN"))
	// start hourly cron to update wiseold man information
	http.HandleFunc("/members", MemberList)
	http.HandleFunc("/activity", Activity)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// for specifically the root / index
		// we want to replace the default with our own
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			clanInfo.Lock()
			templates.Index(clanInfo.Info).Render(context.Background(), w)
			clanInfo.Unlock()
			return
			// do a templ include!!
		}
		http.FileServer(getFileSystem()).ServeHTTP(w, r)
	})
	fmt.Println("Server is running on port 42069")
	fmt.Println(http.ListenAndServe(":42069", nil))
}

func Activity(w http.ResponseWriter, r *http.Request) {
	clanInfo.Lock()
	defer clanInfo.Unlock()
	var Data []route.ActivityBoss
	var Total int
	p := message.NewPrinter(message.MatchLanguage("en"))
	for k, v := range clanInfo.Bosses {
		Data = append(Data, route.ActivityBoss{Name: strings.ReplaceAll(k, "_", " "), Kills: p.Sprint(number.Decimal(v)), KillsNum: v})
		Total += v
	}
	slices.SortFunc(Data, func(i route.ActivityBoss, j route.ActivityBoss) int {
		return cmp.Compare(i.KillsNum, j.KillsNum) * -1
	})
	route.ActivityBossKC(Data, p.Sprint(number.Decimal(Total))).Render(context.Background(), w)
}

func MemberList(w http.ResponseWriter, r *http.Request) {
	// get the member list from the api
	var data []route.ClanMember
	clanInfo.Lock()
	defer clanInfo.Unlock()
	for _, v := range clanInfo.Info.Memberships {
		t, _ := time.Parse(time.RFC3339, v.CreatedAt)
		createDiff := timediff.TimeDiff(t)
		data = append(data, route.ClanMember{
			PlayerID:      v.PlayerID,
			Role:          v.Role,
			CreatedAt:     createDiff,
			CreatedAtUnix: t.Unix(),
			Player: struct {
				Name string "json:\"displayName\""
			}{Name: v.Player.Name},
		})
	}
	slices.SortFunc(data, func(i route.ClanMember, j route.ClanMember) int {
		// lowest first in list
		r := func(a route.ClanMember) int {
			switch a.Role {
			case "owner":
				return RankOwner
			case "deputy_owner":
				return RankDeputyOwner
			case "master":
				return RankMaster
			case "general":
				return RankGeneral
			case "major":
				return RankMajor
			case "proselyte":
				return RankProselyte
			case "zenyte":
				return RankZenyte
			case "onyx":
				return RankOnyx
			case "dragonstone":
				return RankDragonStone
			case "diamond":
				return RankDiamond
			case "ruby":
				return RankRuby
			case "emerald":
				return RankEmerald
			case "sapphire":
				return RankSapphire
			case "member":
				return RankMember
			case "guest":
				return RankGuest
			default:
				return -1
			}
		}
		// compare rank
		c := cmp.Compare(r(i), r(j))
		// if the rank is the same, compare the created at time
		if c == 0 {
			return cmp.Compare(i.CreatedAtUnix, j.CreatedAtUnix)
		}
		return c
	})
	route.MemberList(data).Render(context.Background(), w)
}

type wiseOldMangained struct {
	Data struct {
		Gained int `json:"gained"`
	} `json:"data"`
}

func wiseOldmanSync(token string) {
	rl := rate.NewLimiter(rate.Every(time.Minute/10), 1)
	ctx := context.Background()
	apiBossKC := "https://api.wiseoldman.net/v2/groups/%d/gained?metric=%s&period=week&limit=50&offset=%d"
	for {
		t := time.After(3 * time.Hour)
		// get clan information
		rl.Wait(ctx)
		req, _ := http.NewRequest("GET", "https://api.wiseoldman.net/v2/groups/276", nil)
		if token != "" {
			req.Header.Add("x-api-key", token)
		}
		req.Header.Add("User-Agent", "discordUser/Zanven org/auosrs")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("failed to get clan details", err)
		} else {
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				fmt.Println("Failed to read clan details", err)
			} else {
				clanInfo.Lock()
				json.Unmarshal(data, &clanInfo.Info)
				clanInfo.Unlock()
			}
		}
		// lookup wiseoldman information per boss
		for _, boss := range Bosses {
			offset := 0
			bossResults := make([]wiseOldMangained, 0)
			// loop for the boss until the clan hasn't gained any more kills
			for {
				rl.Wait(ctx)
				fmt.Println("fetching: Boss: ", boss, "Offset: ", offset)

				req, _ := http.NewRequest("GET", fmt.Sprintf(apiBossKC, clanID, boss, offset), nil)
				if token != "" {
					req.Header.Add("x-api-key", token)
				}
				req.Header.Add("User-Agent", "discordUser/Zanven org/auosrs")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					log.Println(err)
					break
				}
				defer resp.Body.Close()
				results := make([]wiseOldMangained, 0)
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					break
				}
				json.Unmarshal(data, &results)
				if len(results) == 0 {
					break
				}
				bossResults = append(bossResults, results...)
				if results[len(results)-1].Data.Gained == 0 {
					break
				}
				offset += 50
			}
			var total int
			for _, v := range bossResults {
				total += v.Data.Gained
			}
			fmt.Println("Boss: ", boss, "Total: ", total)
			// replace the value in the mutex data
			clanInfo.Lock()
			clanInfo.Bosses[boss] = total
			clanInfo.Unlock()
		}
		// make two tables for bosses for 7 days and 30 days
		//
		fmt.Println("fetching completed")
		<-t
	}
}
