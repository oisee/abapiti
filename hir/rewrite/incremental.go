package rewrite

// regionSelection restricts a relation head column to values supplied by the
// adapter. Plans prune a branch as soon as that head term becomes bound.
type regionSelection struct {
	column   int
	contains func(string) bool
}

// dependentRegions computes the conservative reverse dependency closure. Region
// names and the graph come from the adapter; the engine has no IR knowledge.
func dependentRegions(changed map[string]bool, reverse map[string][]string) map[string]bool {
	affected := map[string]bool{}
	var queue []string
	for region := range changed {
		affected[region] = true
		queue = append(queue, region)
	}
	for i := 0; i < len(queue); i++ {
		for _, region := range reverse[queue[i]] {
			if !affected[region] {
				affected[region] = true
				queue = append(queue, region)
			}
		}
	}
	return affected
}

// invalidateRegions removes base and derived facts owned by affected regions.
// Shared derived relations are recomputed conservatively, including negatives;
// immutable global base facts and unrelated regional proofs survive the round.
func (d *DB) invalidateRegions(affected map[string]bool, rules *Rules) {
	derived := map[string]bool{}
	for _, c := range rules.clauses {
		derived[c.head.pred] = true
	}
	for pred, t := range d.tables {
		kept := t.rows[:0]
		for _, row := range t.rows {
			if affected[row.region] || row.region == "" && derived[pred] {
				continue
			}
			kept = append(kept, row)
		}
		if len(kept) == len(t.rows) {
			continue
		}
		clear(t.rows[len(kept):])
		t.rows = kept
		t.keys = map[packedTuple]*row{}
		t.joins = map[uint64]map[packedTuple][]*row{}
		for _, row := range kept {
			t.keys[pack(row.ids, ^uint64(0))] = row
		}
	}
}
