// Select and MultiSelect are deprecated in newer Grafana, but Combobox and
// MultiCombobox are missing from the 11.x releases this plugin supports.
/* eslint-disable @typescript-eslint/no-deprecated */
import React, { useEffect, useMemo, useState } from 'react';
import { InlineField, InlineFieldRow, InlineSwitch, Input, MultiSelect, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { getTemplateSrv } from '@grafana/runtime';
import { DataSource } from '../datasource';
import { DbStatisticFields, HostStatisticFields, SqlStatisticFields } from './StatisticFields';
import { DBmarlinQuery, GroupBy, Metric, MyDataSourceOptions, QueryType, DEFAULT_QUERY } from '../types';

type Props = QueryEditorProps<DataSource, DBmarlinQuery, MyDataSourceOptions>;

const GROUP_BY: Array<SelectableValue<GroupBy>> = [
  { label: 'None', value: 'none' },
  { label: 'Wait event', value: 'waitevent' },
  { label: 'SQL', value: 'sqlhash' },
  { label: 'Session', value: 'session' },
  { label: 'User', value: 'username' },
  { label: 'Database', value: 'database' },
  { label: 'Client', value: 'client' },
  { label: 'Program', value: 'program' },
  { label: 'Module', value: 'module' },
];

const QUERY_TYPES: Array<SelectableValue<QueryType>> = [
  { label: 'Activity time series', value: 'activity_timeseries' },
  { label: 'Activity top N table', value: 'activity_table' },
  { label: 'Activity total per instance', value: 'activity_summary' },
  { label: 'SQL statistics top N table', value: 'sql_statistics' },
  { label: 'Database statistics', value: 'db_statistic' },
  { label: 'Host metrics', value: 'host_statistic' },
];

// A table needs a dimension to list.
const TABLE_GROUP_BY = GROUP_BY.filter((o) => o.value !== 'none');

const METRICS: Array<SelectableValue<Metric>> = [
  { label: 'AAS (average active sessions)', value: 'aas' },
  { label: 'Wait time (s)', value: 'waittime' },
  { label: 'Wait count', value: 'waitcount' },
  { label: 'Avg wait time (ms)', value: 'avgwaittime' },
];

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const q = { ...DEFAULT_QUERY, ...query } as DBmarlinQuery;
  const [instances, setInstances] = useState<Array<SelectableValue<string>>>([]);
  const [error, setError] = useState<string>();

  useEffect(() => {
    datasource
      .getInstances()
      .then(setInstances)
      .catch((e) => setError(e?.data?.message ?? e?.message ?? 'Could not load instances'));
  }, [datasource]);

  // Dashboard variables can stand in for instances, e.g. $instance.
  const options = useMemo(() => {
    const variables = getTemplateSrv()
      .getVariables()
      .map((v) => ({ label: `$${v.name}`, value: `$${v.name}` }));
    return [...variables, ...instances];
  }, [instances]);

  const update = (patch: Partial<DBmarlinQuery>) => {
    onChange({ ...q, ...patch });
    onRunQuery();
  };

  const isTable = q.queryType === 'activity_table';
  const isActivity = isTable || q.queryType === 'activity_timeseries';
  const grouped = isTable || q.groupBy !== 'none';
  const changeType = (queryType: QueryType) =>
    update({
      queryType,
      // a table lists one dimension, so it cannot stay ungrouped
      groupBy: queryType === 'activity_table' && q.groupBy === 'none' ? 'waitevent' : q.groupBy,
    });

  return (
    <div>
      <InlineField label="Query type" labelWidth={16}>
        <Select
          inputId="query-editor-type"
          width={32}
          options={QUERY_TYPES}
          value={q.queryType}
          onChange={(v) => changeType(v.value!)}
        />
      </InlineField>
      <InlineField label="Instances" labelWidth={16} grow invalid={!!error} error={error}>
        <MultiSelect
          inputId="query-editor-instances"
          options={options}
          value={q.instances}
          onChange={(v) => update({ instances: v.map((o) => o.value!) })}
          placeholder="Select instances"
          isLoading={!instances.length && !error}
        />
      </InlineField>
      {q.queryType === 'db_statistic' && <DbStatisticFields query={q} datasource={datasource} update={update} />}
      {q.queryType === 'sql_statistics' && <SqlStatisticFields query={q} datasource={datasource} update={update} />}
      {q.queryType === 'host_statistic' && <HostStatisticFields query={q} datasource={datasource} update={update} />}
      {(q.queryType === 'activity_timeseries' || q.queryType === 'activity_summary') && (
        <InlineFieldRow>
          <InlineField label="Metric" labelWidth={16}>
            <Select
              inputId="query-editor-metric"
              width={32}
              options={METRICS}
              value={q.metric}
              onChange={(v) => update({ metric: v.value! })}
            />
          </InlineField>
          <InlineField label="Combine instances" tooltip="Add the instances together instead of one series each">
            <InlineSwitch
              id="query-editor-aggregate"
              value={q.aggregateInstances}
              onChange={(e) => update({ aggregateInstances: e.currentTarget.checked })}
            />
          </InlineField>
        </InlineFieldRow>
      )}
      {isActivity && (
        <InlineFieldRow>
          <InlineField
            label={isTable ? 'List' : 'Group by'}
            labelWidth={16}
            tooltip={isTable ? undefined : 'Needs DBmarlin 6.12 or later'}
          >
            <Select
              inputId="query-editor-groupby"
              width={20}
              options={isTable ? TABLE_GROUP_BY : GROUP_BY}
              value={q.groupBy}
              onChange={(v) => update({ groupBy: v.value! })}
            />
          </InlineField>
          {grouped && (
            <>
              <InlineField label="Top" tooltip="Number of values to show, ranked by total wait time (at most 50)">
                <Input
                  id="query-editor-topn"
                  type="number"
                  width={8}
                  min={1}
                  max={50}
                  value={q.topN}
                  onChange={(e) => update({ topN: Math.max(1, parseInt(e.currentTarget.value, 10) || 1) })}
                />
              </InlineField>
              {!isTable && (
                <InlineField label="Show other" tooltip="Add everything outside the top values as one 'Other' series">
                  <InlineSwitch
                    id="query-editor-other"
                    value={q.includeOther}
                    onChange={(e) => update({ includeOther: e.currentTarget.checked })}
                  />
                </InlineField>
              )}
            </>
          )}
        </InlineFieldRow>
      )}
    </div>
  );
}
