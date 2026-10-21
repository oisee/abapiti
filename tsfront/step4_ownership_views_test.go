package tsfront

import "testing"

func TestStep4OwnershipOptionalMethodArrayNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Artifact{getName():string;metadata?:unknown}
 interface Files{getFiles():readonly string[]}
 type Owned=Artifact&Partial<Pick<Files,"getFiles">>;
 class Item{getName():string{return "a"}getFiles():readonly string[]{return ["x","y"]}}
 function inspect(objects:readonly Artifact[]):string{const owned:readonly Owned[]=objects;return owned.map(o=>o.getName()+":"+(o.getFiles?.()??[]).join(",")).join(";");}
 class Bare{getName():string{return "b"}}
 class Probe{static run():string{return inspect([new Item(),new Bare()]);}}`)
}
