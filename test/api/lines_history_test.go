package api

import (
	"net/http"
	"testing"
)

type lineEventBody struct {
	Type  string `json:"type"`
	Actor struct {
		Username string `json:"username"`
	} `json:"actor"`
	Params struct {
		Number     string   `json:"number"`
		Line       lineBody `json:"line"`
		SourceLine lineBody `json:"sourceLine"`
	} `json:"params"`
}

func (f versionFixture) history(t *testing.T) []lineEventBody {
	t.Helper()
	var h struct {
		Items []lineEventBody `json:"items"`
	}
	plainUserClient(t, f.env, "bob").Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&h)
	return h.Items
}

func TestForkingAndRegisteringOnAMaintenanceLineAreRecordedInTheChangeHistory(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")
	var line lineDetailBody
	f.fork(src.ID, nil).ExpectStatus(http.StatusCreated).JSON(&line)
	f.fork(src.ID, nil).ExpectStatus(http.StatusUnprocessableEntity) // a rejected fork leaves no event
	f.elarsen.Post(versionsPath("payments-core", line.ID), newVersion(map[string]any{"number": "2.1.0-nw.1", "releasedAt": "2025-04-03"})).
		ExpectStatus(http.StatusCreated)

	h := f.history(t) // newest first: registered on the fork, forked, registered on the mainline, app created
	if len(h) != 4 {
		t.Fatalf("history = %+v", h)
	}
	if e := h[0]; e.Type != "versionRegistered" || e.Params.Number != "2.1.0-nw.1" || e.Params.Line.ID != line.ID ||
		e.Params.Line.Name != "Northwind customization" || e.Params.Line.Branch != "cust/northwind" || e.Params.Line.IsMain {
		t.Fatalf("version event = %+v", e)
	}
	e := h[1]
	if e.Type != "lineForked" || e.Actor.Username != "elarsen" || e.Params.Number != "2.1.0" ||
		e.Params.Line.ID != line.ID || e.Params.Line.Name != "Northwind customization" || e.Params.Line.Branch != "cust/northwind" ||
		e.Params.SourceLine.ID != f.mainline.ID || !e.Params.SourceLine.IsMain {
		t.Fatalf("fork event = %+v", e)
	}
}
