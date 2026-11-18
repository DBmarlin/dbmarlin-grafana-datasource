import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { VariableQuery } from '../variables';

type Props = QueryEditorProps<DataSource, any, any, VariableQuery>;

/** There is nothing to choose yet: the variable lists the instances you can see. */
export function VariableQueryEditor({ query, onChange }: Props) {
  // Store the kind so the saved variable says what it lists.
  React.useEffect(() => {
    if (query.kind !== 'instances') {
      onChange({ ...query, kind: 'instances' });
    }
  }, [query, onChange]);

  return (
    <p>
      Lists the DBmarlin instances this data source can see: the name is shown, the id is the value.
    </p>
  );
}
