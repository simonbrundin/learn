package knowledge

import (
	"sort"
	"strings"

	"github.com/simon/repo-template/internal/model"
)

// Relations som grafen kan innehålla.
const (
	RelationRelated  = "related"
	RelationContrast = "contrast"
	RelationCause    = "cause"
	RelationEffect   = "effect"
)

type contrastMarker struct {
	marker   string
	relation string
}

var contrastMarkers = []contrastMarker{
	{"unlike ", RelationContrast},
	{"in contrast to ", RelationContrast},
	{"in contrast with ", RelationContrast},
	{"whereas ", RelationContrast},
	{"as opposed to ", RelationContrast},
	{"till skillnad från ", RelationContrast},
}

var causeMarkers = []contrastMarker{
	{"because of ", RelationCause},
	{"due to ", RelationCause},
	{"thanks to ", RelationCause},
	{"led to ", RelationEffect},
	{"leads to ", RelationEffect},
	{"results in ", RelationEffect},
	{"resulterar i ", RelationEffect},
	{"leder till ", RelationEffect},
}

// Graph är en in-memory kunskapsgraf.
type Graph struct {
	nodes     map[int64]*model.Concept
	edges     []model.Edge
	adjacency map[int64]map[int64]relationWeight
}

type relationWeight struct {
	relation string
	weight   float64
}

// NewGraph skapar en tom graf.
func NewGraph() *Graph {
	return &Graph{
		nodes:     map[int64]*model.Concept{},
		adjacency: map[int64]map[int64]relationWeight{},
	}
}

// AddNode lägger till ett koncept i grafen.
func (g *Graph) AddNode(concept model.Concept) {
	g.nodes[concept.ID] = &concept
}

// AddEdge kopplar ihop två koncept.
func (g *Graph) AddEdge(sourceID, targetID int64, relation string, weight float64, bookID int64) {
	if sourceID == targetID {
		return
	}
	if _, ok := g.nodes[sourceID]; !ok {
		return
	}
	if _, ok := g.nodes[targetID]; !ok {
		return
	}
	if existing, ok := g.adjacency[sourceID][targetID]; ok {
		if existing.weight >= weight {
			return
		}
		g.adjacency[sourceID][targetID] = relationWeight{relation, weight}
		g.adjacency[targetID][sourceID] = relationWeight{relation, weight}
		for i := range g.edges {
			if (g.edges[i].SourceID == sourceID && g.edges[i].TargetID == targetID) ||
				(g.edges[i].SourceID == targetID && g.edges[i].TargetID == sourceID) {
				g.edges[i].Relation = relation
				g.edges[i].Weight = weight
				return
			}
		}
		return
	}
	if g.adjacency[sourceID] == nil {
		g.adjacency[sourceID] = map[int64]relationWeight{}
	}
	if g.adjacency[targetID] == nil {
		g.adjacency[targetID] = map[int64]relationWeight{}
	}
	g.adjacency[sourceID][targetID] = relationWeight{relation, weight}
	g.adjacency[targetID][sourceID] = relationWeight{relation, weight}
	g.edges = append(g.edges, model.Edge{
		SourceID: sourceID,
		TargetID: targetID,
		Relation: relation,
		Weight:   weight,
		BookID:   bookID,
	})
}

// BuildFromContexts bygger relationer utifrån meningarna där koncepten nämns.
func (g *Graph) BuildFromContexts(concepts []model.Concept, contexts map[string][]string, bookID int64) {
	for _, concept := range concepts {
		g.AddNode(concept)
	}

	byKey := map[string]*model.Concept{}
	for i := range concepts {
		byKey[strings.ToLower(concepts[i].Name)] = &concepts[i]
	}

	type pair struct {
		source, target int64
	}
	weights := map[pair]float64{}
	hints := map[pair]string{}

	for key, sentences := range contexts {
		concept, ok := byKey[key]
		if !ok {
			continue
		}
		for _, sentence := range sentences {
			lowered := strings.ToLower(sentence)
			for otherKey, other := range byKey {
				if otherKey == key || !strings.Contains(lowered, otherKey) {
					continue
				}
				p := pair{source: concept.ID, target: other.ID}
				if p.source > p.target {
					p.source, p.target = p.target, p.source
				}
				weights[p]++
				if _, exists := hints[p]; !exists {
					if hint := detectRelation(lowered, key, otherKey); hint != "" {
						hints[p] = hint
					}
				}
			}
		}
	}

	for p, weight := range weights {
		relation := RelationRelated
		if hint, ok := hints[p]; ok {
			relation = hint
		}
		g.AddEdge(p.source, p.target, relation, weight, bookID)
	}
}

func detectRelation(sentence, key, other string) string {
	for _, marker := range contrastMarkers {
		if strings.Contains(sentence, marker.marker) && strings.Contains(sentence, other) {
			return marker.relation
		}
	}
	for _, marker := range causeMarkers {
		if strings.Contains(sentence, marker.marker) && strings.Contains(sentence, other) {
			return marker.relation
		}
	}
	return ""
}

// Neighbour är en grann med relation och styrka.
type Neighbour struct {
	Concept  model.Concept
	Relation string
	Weight   float64
}

// Neighbours returnerar konceptets grannar, starkast först.
func (g *Graph) Neighbours(conceptID int64) []Neighbour {
	var result []Neighbour
	for otherID, relation := range g.adjacency[conceptID] {
		concept, ok := g.nodes[otherID]
		if !ok {
			continue
		}
		result = append(result, Neighbour{Concept: *concept, Relation: relation.relation, Weight: relation.weight})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Weight != result[j].Weight {
			return result[i].Weight > result[j].Weight
		}
		return result[i].Concept.Name < result[j].Concept.Name
	})
	return result
}

// Related rankar grannar med vikt plus delade nyckelord.
func (g *Graph) Related(concept model.Concept, limit int) []Neighbour {
	keywords := map[string]bool{}
	for _, keyword := range concept.Keywords {
		keywords[strings.ToLower(keyword)] = true
	}
	neighbours := g.Neighbours(concept.ID)
	for i := range neighbours {
		overlap := 0
		for _, keyword := range neighbours[i].Concept.Keywords {
			if keywords[strings.ToLower(keyword)] {
				overlap++
			}
		}
		neighbours[i].Weight += float64(overlap) * 0.25
	}
	sort.Slice(neighbours, func(i, j int) bool {
		if neighbours[i].Weight != neighbours[j].Weight {
			return neighbours[i].Weight > neighbours[j].Weight
		}
		return neighbours[i].Concept.Name < neighbours[j].Concept.Name
	})
	if len(neighbours) > limit {
		neighbours = neighbours[:limit]
	}
	return neighbours
}

// FindPath söker en kopplingsväg mellan två koncept.
func (g *Graph) FindPath(sourceID, targetID int64) []model.Concept {
	if _, ok := g.nodes[sourceID]; !ok {
		return nil
	}
	if _, ok := g.nodes[targetID]; !ok {
		return nil
	}
	if sourceID == targetID {
		return []model.Concept{*g.nodes[sourceID]}
	}
	visited := map[int64]bool{sourceID: true}
	queue := [][]int64{{sourceID}}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		current := path[len(path)-1]
		neighbourIDs := make([]int64, 0, len(g.adjacency[current]))
		for id := range g.adjacency[current] {
			neighbourIDs = append(neighbourIDs, id)
		}
		sort.Slice(neighbourIDs, func(i, j int) bool { return neighbourIDs[i] < neighbourIDs[j] })
		for _, neighbour := range neighbourIDs {
			if visited[neighbour] {
				continue
			}
			newPath := append(append([]int64{}, path...), neighbour)
			if neighbour == targetID {
				result := make([]model.Concept, 0, len(newPath))
				for _, id := range newPath {
					result = append(result, *g.nodes[id])
				}
				return result
			}
			visited[neighbour] = true
			queue = append(queue, newPath)
		}
	}
	return nil
}

// CrossBookLinks hittar liknande koncept i olika böcker.
func (g *Graph) CrossBookLinks(concepts []model.Concept) [][2]model.Concept {
	grouped := map[string][]model.Concept{}
	for _, concept := range concepts {
		key := strings.ToLower(concept.Name)
		grouped[key] = append(grouped[key], concept)
	}
	var links [][2]model.Concept
	for _, group := range grouped {
		if len(group) < 2 {
			continue
		}
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				if group[i].BookID == group[j].BookID {
					continue
				}
				links = append(links, [2]model.Concept{group[i], group[j]})
			}
		}
	}
	return links
}

// Node returnerar konceptet med givet id.
func (g *Graph) Node(id int64) (model.Concept, bool) {
	concept, ok := g.nodes[id]
	if !ok {
		return model.Concept{}, false
	}
	return *concept, true
}

// Stats ger antal koncept och relationer.
func (g *Graph) Stats() (concepts int, edges int) {
	return len(g.nodes), len(g.edges)
}

// Edges returnerar grafens relationer.
func (g *Graph) Edges() []model.Edge { return g.edges }
