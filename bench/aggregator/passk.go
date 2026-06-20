package aggregator

import "math"

// PassAtK computes the HumanEval UNBIASED pass@k estimator for a single task:
//
//	pass@k = 1 − C(n−c, k) / C(n, k)
//
// where n = number of runs for the task, c = number of those runs with
// task_success == true, and k = the target sample budget. This is the locked
// estimator (D-10); the naive 1 − (1 − c/n)^k is FORBIDDEN — it is biased and
// fails published reference values at k>=2 (it gives 0.83193 for (10,3,5)
// instead of the correct 0.91667).
//
// pass@1 reduces exactly to c/n (the success-rate identity), so the same
// per-task reduction that feeds the boolean task_success leaderboard column IS
// pass@1.
//
// Implemented in Chen et al.'s numerically-stable product form, which never
// computes factorials and never overflows:
//
//	C(n−c,k)/C(n,k) = Π_{i=n−c+1}^{n} (i−k)/i = Π_{i=n−c+1}^{n} (1 − k/i)
//
// [CITED: Chen et al. 2021, "Evaluating Large Language Models Trained on Code",
// arXiv:2107.03374, §2.1]
func PassAtK(n, c, k int) float64 {
	// Guard k > n per D-11. Callers pass k<=n; if k exceeds the available
	// samples the estimator is undefined, so we fold it into the n-c<k branch
	// (return 1.0) rather than producing a nonsense product.
	if k > n {
		return 1.0
	}
	if n-c < k {
		// Fewer than k incorrect samples → any k samples must include at least
		// one correct one.
		return 1.0
	}
	prod := 1.0
	// 1 − C(n-c,k)/C(n,k) computed as 1 − Π_{i=n-c+1}^{n} (1 − k/i).
	//
	// CRITICAL: the loop runs i = n-c+1 .. n, which is exactly
	// (n - (n-c+1) + 1) = c terms. The term count is c (= number of
	// successes), NOT k — looping k times is the classic wrong implementation
	// (it yields 0.97348 for (n=10,c=3,k=5) instead of the correct 0.91667).
	for i := n - c + 1; i <= n; i++ {
		prod *= 1.0 - float64(k)/float64(i)
	}
	return 1.0 - prod
}

// logBinom returns ln C(a, b) computed via math.Lgamma (log-gamma), returning
// -Inf for b<0 || b>a. The product form in PassAtK is PRIMARY; logBinom exists
// for the D-11 cross-check (passk_test.go computes pass@k independently in
// log-space and asserts agreement) and is available to callers wanting a
// log-space path. Both derivations implement the same closed form and agree.
func logBinom(a, b int) float64 {
	if b < 0 || b > a {
		return math.Inf(-1)
	}
	la, _ := math.Lgamma(float64(a) + 1)
	lb, _ := math.Lgamma(float64(b) + 1)
	lab, _ := math.Lgamma(float64(a-b) + 1)
	return la - lb - lab
}
