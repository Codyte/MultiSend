package split

import "fmt"

func ComputeParts(total, leftWeight, rightWeight int64) (int64, int64, error) {
	if total < 0 || leftWeight <= 0 || rightWeight <= 0 {
		return 0, 0, fmt.Errorf("invalid inputs: total=%d, lw=%d, rw=%d", total, leftWeight, rightWeight)
	}
	sum := leftWeight + rightWeight
	left := (total * leftWeight) / sum
	right := total - left
	return left, right, nil
}
