package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LatencyDistribution uses milliseconds and nearest-rank quantiles. N=0 means
// unavailable; unstarted attempts are never represented as zero-latency samples.
type LatencyDistribution struct {
	N    int     `json:"n"`
	Mean float64 `json:"mean_ms"`
	P50  float64 `json:"p50_ms"`
	P95  float64 `json:"p95_ms"`
	Min  float64 `json:"min_ms"`
	Max  float64 `json:"max_ms"`
}

func latencyDistribution(values []float64) LatencyDistribution {
	if len(values) == 0 {
		return LatencyDistribution{}
	}
	sort.Float64s(values)
	d := LatencyDistribution{N: len(values), Min: values[0], Max: values[len(values)-1]}
	for _, v := range values {
		d.Mean += v / float64(d.N)
	}
	d.P50 = values[int(math.Ceil(.50*float64(d.N)))-1]
	d.P95 = values[int(math.Ceil(.95*float64(d.N)))-1]
	return d
}

type ReliabilityMetrics struct {
	Attempts    int     `json:"attempts"`
	Successes   int     `json:"successes"`
	SuccessRate float64 `json:"success_rate"`
	// Wilson 95% interval describes sampling uncertainty, not equivalence.
	SuccessLow  float64        `json:"success_wilson95_low"`
	SuccessHigh float64        `json:"success_wilson95_high"`
	Failures    map[string]int `json:"failures"`
	// MixedOutcomes identifies observed flakiness; false is not proof of reliability.
	MixedOutcomes          bool `json:"mixed_outcomes"`
	FingerprintedSuccesses int  `json:"fingerprinted_successes"`
	DistinctResults        int  `json:"distinct_results"`
	// Nil until at least two successful results have fingerprints. This is the
	// fraction of pairs with identical task-defined semantic results.
	ResultAgreement *float64 `json:"result_pair_agreement,omitempty"`
}

type Metrics struct {
	Wall           LatencyDistribution `json:"wall"`
	SuccessfulWall LatencyDistribution `json:"successful_wall"`
	Agent          LatencyDistribution `json:"agent"`
	Verify         LatencyDistribution `json:"verify"`
	Reliability    ReliabilityMetrics  `json:"reliability"`
}

func (tr TaskResult) Metrics() Metrics {
	var wall, success, agent, verify []float64
	r := ReliabilityMetrics{Attempts: len(tr.Rollouts), Failures: map[string]int{}}
	hashes := map[string]int{}
	for _, attempt := range tr.Rollouts {
		if attempt.WallMS != nil {
			wall = append(wall, *attempt.WallMS)
		}
		if attempt.AgentMS != nil {
			agent = append(agent, *attempt.AgentMS)
		}
		if attempt.VerifyMS != nil {
			verify = append(verify, *attempt.VerifyMS)
		}
		if attempt.Verified {
			r.Successes++
			if attempt.WallMS != nil {
				success = append(success, *attempt.WallMS)
			}
			if attempt.ResultDigest != "" {
				hashes[attempt.ResultDigest]++
				r.FingerprintedSuccesses++
			}
		} else {
			reason := attempt.FailureReason
			if reason == "" {
				reason = "unknown"
			}
			r.Failures[reason]++
		}
	}
	if r.Attempts > 0 {
		n := float64(r.Attempts)
		r.SuccessRate = float64(r.Successes) / n
		const z = 1.959963984540054
		denom := 1 + z*z/n
		center := (r.SuccessRate + z*z/(2*n)) / denom
		half := z * math.Sqrt(r.SuccessRate*(1-r.SuccessRate)/n+z*z/(4*n*n)) / denom
		r.SuccessLow, r.SuccessHigh = math.Max(0, center-half), math.Min(1, center+half)
	}
	r.MixedOutcomes = r.Successes > 0 && r.Successes < r.Attempts
	r.DistinctResults = len(hashes)
	if r.FingerprintedSuccesses >= 2 {
		var same float64
		for _, count := range hashes {
			same += float64(count) * float64(count-1)
		}
		n := float64(r.FingerprintedSuccesses)
		agreement := same / (n * (n - 1))
		r.ResultAgreement = &agreement
	}
	return Metrics{Wall: latencyDistribution(wall), SuccessfulWall: latencyDistribution(success), Agent: latencyDistribution(agent), Verify: latencyDistribution(verify), Reliability: r}
}

func resultDigest(task Task) (string, error) {
	h := sha256.New()
	enc := json.NewEncoder(h)
	root, err := filepath.EvalSymlinks(task.RepoDir)
	if err != nil {
		return "", err
	}
	for _, name := range task.ResultFiles {
		if !filepath.IsLocal(name) {
			return "", fmt.Errorf("result file must be relative: %q", name)
		}
		path, err := filepath.EvalSymlinks(filepath.Join(root, name))
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("result file escapes episode: %q", name)
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("result file is not regular: %q", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if err := enc.Encode(struct {
			Name string
			Data []byte
		}{name, data}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
