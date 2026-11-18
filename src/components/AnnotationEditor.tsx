// See QueryEditor.tsx for why the deprecated selects are used.
/* eslint-disable @typescript-eslint/no-deprecated */
import React, { useEffect, useState } from 'react';
import { InlineField, MultiSelect } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { DBmarlinQuery, EventKind, MyDataSourceOptions } from '../types';

type Props = QueryEditorProps<DataSource, DBmarlinQuery, MyDataSourceOptions>;

const KINDS: Array<SelectableValue<EventKind>> = [
  { label: 'Alerts', value: 'alert' },
  { label: 'Changes', value: 'change' },
  { label: 'Custom events (e.g. deployments)', value: 'custom' },
];

/** Which instances' events become annotations, and of which kinds. */
export function AnnotationEditor({ query, onChange, datasource }: Props) {
  const [instances, setInstances] = useState<Array<SelectableValue<string>>>([]);
  const [error, setError] = useState<string>();

  useEffect(() => {
    datasource
      .getInstances()
      .then(setInstances)
      .catch((e) => setError(e?.data?.message ?? e?.message ?? 'Could not load instances'));
  }, [datasource]);

  const update = (patch: Partial<DBmarlinQuery>) => onChange({ ...query, queryType: 'events', ...patch });

  return (
    <div>
      <InlineField label="Instances" labelWidth={16} grow invalid={!!error} error={error}>
        <MultiSelect
          inputId="annotation-editor-instances"
          options={instances}
          value={query.instances ?? []}
          onChange={(v) => update({ instances: v.map((o) => o.value!) })}
          placeholder="Select instances"
          allowCustomValue
          isLoading={!instances.length && !error}
        />
      </InlineField>
      <InlineField label="Show" labelWidth={16} grow tooltip="Events on the instances and on their hosts">
        <MultiSelect
          inputId="annotation-editor-kinds"
          options={KINDS}
          value={query.eventKinds ?? []}
          onChange={(v) => update({ eventKinds: v.map((o) => o.value!) })}
          placeholder="All kinds"
        />
      </InlineField>
    </div>
  );
}
