package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// Structures pour le modèle TALN
type ModelBigramme map[string]map[string]int
type Dictionnaire map[string]int

type CorrectionResponse struct {
	Original string `json:"original"`
	Corrige  string `json:"corrige"`
}
type IndexSymSpell map[string][]string

var dict Dictionnaire
var bigrams ModelBigramme

var index = make(IndexSymSpell)

func SauvegarderModele(nomFichier string) {
	file, err := os.Create(nomFichier)
	if err != nil {
		fmt.Println("Erreur création fichier sauvegarde:", err)
		return
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	// On stocke dict et bigrams dans une structure ou séparement
	encoder.Encode(dict)
	encoder.Encode(bigrams)
	fmt.Println("💾 Modèle sauvegardé avec succès.")
}

// Charger le modèle depuis le disque au démarrage
func ChargerModele(nomFichier string) bool {
	file, err := os.Open(nomFichier)
	if err != nil {
		return false // Le fichier n'existe pas encore
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	decoder.Decode(&dict)
	decoder.Decode(&bigrams)

	// Re-générer l'index SymSpell à partir du dict chargé
	// for mot := range dict {
	// 	indexerMot(mot)
	// }
	fmt.Println("⚡ Modèle chargé depuis le disque.")
	return true
}
func GenererDeletions(mot string, distanceMax int) []string {
	res := []string{mot}
	// Logique simplifiée pour 1 suppression
	for i := 0; i < len(mot); i++ {
		del := mot[:i] + mot[i+1:]
		res = append(res, del)
	}
	return res
}

// InitialiserIndex remplit l'index ultra-rapide
func InitialiserIndex() {
	for mot := range dict {
		deletions := GenererDeletions(mot, 1)
		for _, d := range deletions {
			index[d] = append(index[d], mot)
		}
	}
}

func RechercherCandidats(input string) []string {
	// On cherche directement dans la map (O(1) au lieu de O(N))
	candidats := GenererDeletions(input, 1)
	suggestions := []string{}

	for _, c := range candidats {
		if val, existe := index[c]; existe {
			suggestions = append(suggestions, val...)
		}
	}
	return suggestions
}
func Tokenize(texte string) []string {
	// 1. Mise en minuscule
	texte = strings.ToLower(texte)

	// 2. Remplacer les apostrophes courbes par des apostrophes simples
	texte = strings.ReplaceAll(texte, "’", "'")

	// 3. Regex pour séparer la ponctuation et les élisions
	// Cette regex capture :
	// - Les mots avec apostrophes (l', d', etc.)
	// - Les mots avec tirets (peut-être)
	// - Les mots simples
	// - La ponctuation isolée
	re := regexp.MustCompile(`[a-zàâçéèêëîïôûùœæ']+(?:-[a-zàâçéèêëîïôûùœæ']+)*|[[:punct:]]`)

	return re.FindAllString(texte, -1)
}

func TokenizePro(texte string) []string {
	texte = strings.ToLower(texte)

	// 1. Définition des patterns
	// Pattern pour Emails
	const emailPattern = `[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`
	// Pattern pour URLs (http, https, www)
	const urlPattern = `(?:https?://|www\.)[^\s/$.?#].[^\s]*`
	// Pattern pour Mots (incluant apostrophes et tirets)
	const wordPattern = `[a-zàâçéèêëîïôûùœæ']+(?:-[a-zàâçéèêëîïôûùœæ']+)*`
	// Pattern pour Ponctuation
	const punctPattern = `[[:punct:]]`

	// On combine tout : le "|" signifie "OU"
	// L'ordre est crucial : on cherche d'abord les URLs, puis Emails, puis Mots
	fullRegex := fmt.Sprintf(`(%s)|(%s)|(%s)|(%s)`, urlPattern, emailPattern, wordPattern, punctPattern)
	re := regexp.MustCompile(fullRegex)

	return re.FindAllString(texte, -1)
}

// --- LOGIQUE TALN ---
// EntrainerDepuisTexte lit un texte brut et met à jour le dictionnaire et les bigrammes
func EntrainerDepuisTexte(chemin string) error {
	file, err := os.Open(chemin)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var motPrecedent string

	for scanner.Scan() {
		ligne := strings.ToLower(scanner.Text())
		// Nettoyage simple de la ponctuation
		ligne = strings.NewReplacer(",", "", ".", "", "!", "", "?", "").Replace(ligne)
		// mots := strings.Fields(ligne)
		mots := TokenizePro(ligne)

		for _, mot := range mots {
			// Mise à jour Unigramme (fréquence du mot seul)
			dict[mot]++

			// Mise à jour Bigramme (fréquence de la suite de mots)
			if motPrecedent != "" {
				if bigrams[motPrecedent] == nil {
					bigrams[motPrecedent] = make(map[string]int)
				}
				bigrams[motPrecedent][mot]++
			}
			motPrecedent = mot
		}
	}
	return nil
}

// Algorithme de Levenshtein pour mesurer la similarité entre deux mots
func Levenshtein(s1, s2 string) int {
	d := make([][]int, len(s1)+1)
	for i := range d {
		d[i] = make([]int, len(s2)+1)
	}
	for i := 0; i <= len(s1); i++ {
		d[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		d[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}
	return d[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
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
		// candidats = RechercherCandidats(mot)
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
	// dict = make(Dictionnaire)
	// bigrams = make(ModelBigramme)

	// // Chargement Unigrammes
	// f, _ := os.Open("dictionnaire.txt")
	// s := bufio.NewScanner(f)
	// for s.Scan() {
	// 	p := strings.Fields(s.Text())
	// 	if len(p) == 2 {
	// 		freq, _ := strconv.Atoi(p[1])
	// 		dict[p[0]] = freq
	// 	}
	// }

	// // Chargement Bigrammes
	// f2, _ := os.Open("bigrammes.txt")
	// s2 := bufio.NewScanner(f2)
	// for s2.Scan() {
	// 	p := strings.Fields(s2.Text())
	// 	if len(p) == 3 {
	// 		if bigrams[p[0]] == nil { bigrams[p[0]] = make(map[string]int) }
	// 		freq, _ := strconv.Atoi(p[2])
	// 		bigrams[p[0]][p[1]] = freq
	// 	}
	// }
	// fmt.Println("✅ Modèles TALN chargés en mémoire.")
	InitialiserIndex()
	dict = make(Dictionnaire)
	bigrams = make(ModelBigramme)

	if !ChargerModele("modele_taln.gob") {
		fmt.Println("⚠️ Aucun modèle trouvé. Lancement de l'entraînement initial...")

		// 2. Entraîner sur ton dossier de données
		dataContent, err := os.ReadDir("data")
		if err != nil {
			panic(err)
		}
		fmt.Println("⏳ Entraînement en cours...")
		for _, filename := range dataContent {
			EntrainerDepuisTexte("data/" + filename.Name())
		}

		// 3. Sauvegarder pour la prochaine fois
		SauvegarderModele("modele_taln.gob")
	}
	// EntrainerDepuisTexte("dico.txt")
	fmt.Printf("✅ Terminé ! %d mots uniques appris.\n", len(dict))
}

func main() {
	http.HandleFunc("/correct", handleCorrection)
	fmt.Println("🚀 API démarrée sur http://localhost:8080/correct")
	http.ListenAndServe(":8080", nil)
}
