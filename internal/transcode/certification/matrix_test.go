package certification

import "testing"

func TestBuildMatrixReportCalculatesControlledDeltas(t *testing.T) {
	legacy := validReport(FixtureCFR48K)
	legacy.Handoff.VideoPresentationDeltaMicros = -33_333
	legacy.Handoff.Video.PresentationDeltaMicros = -33_333
	legacy.Handoff.AudioPresentationDeltaMicros = -104_000
	legacy.Handoff.Audio.PresentationDeltaMicros = -104_000

	production48 := validReport(FixtureCFR48KZeroLatency)
	production48.Handoff.VideoPresentationDeltaMicros = 0
	production48.Handoff.Video.PresentationDeltaMicros = 0
	production48.Handoff.AudioPresentationDeltaMicros = -104_000
	production48.Handoff.Audio.PresentationDeltaMicros = -104_000

	production44 := validReport(FixtureCFR44K1ZeroLatency)
	production44.Handoff.VideoPresentationDeltaMicros = 0
	production44.Handoff.Video.PresentationDeltaMicros = 0
	production44.Handoff.AudioPresentationDeltaMicros = -116_100
	production44.Handoff.Audio.PresentationDeltaMicros = -116_100

	matrix, err := BuildMatrixReport([]Report{production44, legacy, production48})
	if err != nil {
		t.Fatal(err)
	}
	if got := matrix.Comparisons[0].VideoPresentationDeltaChangeMicros; got != 33_333 {
		t.Fatalf("unexpected zerolatency video delta change %d", got)
	}
	if got := matrix.Comparisons[1].AudioPresentationDeltaChangeMicros; got != -12_100 {
		t.Fatalf("unexpected sample-rate audio delta change %d", got)
	}
}

func TestMatrixValidationRejectsMissingFixture(t *testing.T) {
	_, err := BuildMatrixReport([]Report{
		validReport(FixtureCFR48K),
		validReport(FixtureCFR48KZeroLatency),
	})
	if err == nil {
		t.Fatal("incomplete fixture matrix was accepted")
	}
}

func TestMarshalMatrixReportIsNewlineTerminated(t *testing.T) {
	matrix, err := BuildMatrixReport([]Report{
		validReport(FixtureCFR48K),
		validReport(FixtureCFR48KZeroLatency),
		validReport(FixtureCFR44K1ZeroLatency),
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := MarshalMatrixReport(matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 || content[len(content)-1] != '\n' {
		t.Fatal("fixture matrix is not newline terminated")
	}
}
