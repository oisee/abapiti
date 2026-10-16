package golang

import "github.com/oisee/abapiti/hir"

func init() {
	for _, op := range []string{"bigint.decimal", "bigint.neg", "bigint.toString", "bigint.compare", "dynamic.asBigInt"} {
		supportedOps[op] = true
	}
}

func (b *body) bigintRuntime(x *hir.Expr) (string, bool) {
	switch x.Op {
	case "bigint.decimal":
		return "parseBigInt(" + b.expr(x.X) + ")", true
	case "bigint.neg":
		return "negBigInt(" + b.expr(x.X) + ")", true
	case "bigint.toString":
		return "str(string(" + b.expr(x.X) + "))", true
	case "bigint.compare":
		return "compareBigInt(" + b.expr(x.X) + "," + b.expr(x.Args[0]) + ")", true
	case "dynamic.asBigInt":
		return "dynBigInt(" + b.expr(x.X) + ")", true
	}
	return "", false
}
