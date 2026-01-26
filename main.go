package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Structures pour le modèle TALN
type ModelBigramme map[string]map[string]int
type Dictionnaire map[string]int

type CorrectionResponse struct {
	Original string `json:"original"`
	Corrige  string `json:"corrige"`
}

var dict Dictionnaire
var bigrams ModelBigramme

// --- LOGIQUE TALN ---

// Algorithme de Levenshtein pour mesurer la similarité entre deux mots
func Levenshtein(s1, s2 string) int {
	d := make([][]int, len(s1)+1)
	for i := range d { d[i] = make([]int, len(s2)+1) }
	for i := 0; i <= len(s1); i++ { d[i][0] = i }
	for j := 0; j <= len(s2); j++ { d[0][j] = j }

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] { cost = 0 }
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}
	return d[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b && a < c { return a }
	if b < c { return b }
	return c
}

// Fonction principale de correction d'une phrase
func CorrigerPhrase(input string) string {
	mots := strings.Fields(strings.ToLower(input))
	resultat := make([]string, len(mots))

	for i, mot := range mots {
		// 1. Si le mot existe déjà, on le valide
		if _, existe := dict[mot]; existe {
			resultat[i] = mot
			continue
		}

		// 2. Recherche de candidats proches (Distance <= 2)
		candidats := []string{}
		for k := range dict {
			if Levenshtein(mot, k) <= 2 {
				candidats = append(candidats, k)
			}
		}

		// 3. Choix du meilleur candidat par contexte (Bigrammes) ou fréquence
		meilleurCandidat := mot
		maxScore := -1

		for _, c := range candidats {
			score := dict[c] // Score de base par fréquence unigramme
			
			// Si on a un mot précédent, on vérifie la probabilité du bigramme
			if i > 0 {
				precedent := resultat[i-1]
				if s, ok := bigrams[precedent][c]; ok {
					score += s * 1000 // On booste énormément le score si le contexte concorde
				}
			}

			if score > maxScore {
				maxScore = score
				meilleurCandidat = c
			}
		}
		resultat[i] = meilleurCandidat
	}
	return strings.Join(resultat, " ")
}

// --- SERVEUR API ---

func handleCorrection(w http.ResponseWriter, r *http.Request) {
	// Autoriser les requêtes depuis Angular (CORS)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	text := r.URL.Query().Get("text")
	if text == "" {
		json.NewEncoder(w).Encode(CorrectionResponse{Original: "", Corrige: ""})
		return
	}

	corrige := CorrigerPhrase(text)
	json.NewEncoder(w).Encode(CorrectionResponse{Original: text, Corrige: corrige})
}

// --- INITIALISATION ---

func init() {
	dict = make(Dictionnaire)
	bigrams = make(ModelBigramme)

	// Chargement Unigrammes
	f, _ := os.Open("dictionnaire.txt")
	s := bufio.NewScanner(f)
	for s.Scan() {
		p := strings.Fields(s.Text())
		if len(p) == 2 {
			freq, _ := strconv.Atoi(p[1])
			dict[p[0]] = freq
		}
	}

	// Chargement Bigrammes
	f2, _ := os.Open("bigrammes.txt")
	s2 := bufio.NewScanner(f2)
	for s2.Scan() {
		p := strings.Fields(s2.Text())
		if len(p) == 3 {
			if bigrams[p[0]] == nil { bigrams[p[0]] = make(map[string]int) }
			freq, _ := strconv.Atoi(p[2])
			bigrams[p[0]][p[1]] = freq
		}
	}
	fmt.Println("✅ Modèles TALN chargés en mémoire.")
}

func main() {
	http.HandleFunc("/correct", handleCorrection)
	fmt.Println("🚀 API démarrée sur http://localhost:8080/correct")
	http.ListenAndServe(":8080", nil)
}