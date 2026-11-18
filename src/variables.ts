import { CustomVariableSupport, DataQueryRequest, DataQueryResponse } from '@grafana/data';
import { DataQuery } from '@grafana/schema';
import { from, map, Observable } from 'rxjs';
import { DataSource } from './datasource';
import { VariableQueryEditor } from './components/VariableQueryEditor';

/** A dashboard variable query. It only lists instances for now. */
export interface VariableQuery extends DataQuery {
  kind?: 'instances';
}

/** Lets a dashboard variable list the DBmarlin instances (text is the name, value the id). */
export class VariableSupport extends CustomVariableSupport<DataSource, VariableQuery> {
  editor = VariableQueryEditor;

  constructor(private readonly datasource: DataSource) {
    super();
  }

  query(_: DataQueryRequest<VariableQuery>): Observable<DataQueryResponse> {
    return from(this.datasource.getInstances()).pipe(
      map((instances) => ({ data: instances.map((i) => ({ text: i.label, value: i.value })) }))
    );
  }
}
