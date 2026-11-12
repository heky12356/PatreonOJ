// rebuild_graph previews and applies bounded automatic relationships to an initialized database.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"patreonoj/internal/config"
	"patreonoj/internal/models"
	"patreonoj/internal/services/question"
)

func main() {
	configFile := flag.String("config", "config.yaml", "application config")
	protectedFile := flag.String("protected", "", "private JSON pack containing protected edges (from/to/type)")
	prefix := flag.String("legacy-prefix", "", "classify matching legacy auto edges only in this import namespace")
	apply := flag.Bool("apply", false, "commit; default previews and rolls back relationship changes")
	flag.Parse()
	var protected []models.QuestionRelationEdge
	if *protectedFile != "" {
		b, err := os.ReadFile(*protectedFile)
		if err != nil {
			log.Fatal(err)
		}
		var pack struct {
			Edges []struct {
				From string                          `json:"from"`
				To   string                          `json:"to"`
				Type models.QuestionRelationEdgeType `json:"type"`
			} `json:"edges"`
		}
		if err := json.Unmarshal(b, &pack); err != nil {
			log.Fatal("invalid protected edge manifest")
		}
		for _, edge := range pack.Edges {
			if edge.From == "" || edge.To == "" || !edge.Type.Valid() {
				log.Fatal("invalid protected edge")
			}
			protected = append(protected, models.QuestionRelationEdge{FromQuestionID: edge.From, ToQuestionID: edge.To, EdgeType: edge.Type})
		}
	}
	if err := config.InitConfig(*configFile); err != nil {
		log.Fatal(err)
	}
	if err := models.InitDB(); err != nil {
		log.Fatal(err)
	}
	sql, err := models.DB.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sql.Close()
	report, err := question.RebuildAutomaticGraph(models.DB, protected, *prefix, *apply)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		log.Fatal(err)
	}
}
