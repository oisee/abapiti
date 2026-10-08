import {seq, Expression} from "../combi";
import {SQLFromBody} from "./sql_from_body";
import {IStatementRunnable} from "../statement_runnable";

export class SQLFrom extends Expression {
  public getRunnable(): IStatementRunnable {
    return seq("FROM", new SQLFromBody());
  }
}
