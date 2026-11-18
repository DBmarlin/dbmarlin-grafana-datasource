// See QueryEditor.tsx for why the deprecated selects are used.
/* eslint-disable @typescript-eslint/no-deprecated */
import React, { useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, MultiSelect, Select } from '@grafana/ui';
import { SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { DBmarlinQuery, HostMetric } from '../types';

interface Props {
  query: DBmarlinQuery;
  datasource: DataSource;
  update: (patch: Partial<DBmarlinQuery>) => void;
}

const HOST_METRICS: Array<SelectableValue<HostMetric>> = [
  { label: 'CPU utilisation', value: 'cpuutilisation' },
  { label: 'Memory utilisation', value: 'memoryutilisation' },
  { label: 'I/O utilisation', value: 'ioutilisation' },
  { label: 'Disk utilisation', value: 'diskutilisation' },
];

// The first real instance decides which catalogue or hosts to offer; a
// dashboard variable such as $instance cannot be resolved here.
const firstInstance = (query: DBmarlinQuery) => query.instances.find((i) => /^\d+$/.test(i));

/** Pickers for the db_statistic query type. */
export function DbStatisticFields({ query, datasource, update }: Props) {
  const [options, setOptions] = useState<Array<SelectableValue<string>>>([]);
  const [error, setError] = useState<string>();
  const instance = firstInstance(query);

  useEffect(() => {
    if (!instance) {
      return;
    }
    datasource
      .getStatistics(instance)
      .then((o) => {
        setOptions(o);
        setError(undefined);
      })
      .catch((e) => setError(e?.data?.message ?? e?.message ?? 'Could not load statistics'));
  }, [datasource, instance]);

  return (
    <InlineField
      label="Statistics"
      labelWidth={16}
      grow
      invalid={!!error}
      error={error}
      tooltip="Offered for the first selected instance's database type; at most 10"
    >
      <MultiSelect
        inputId="query-editor-statistics"
        options={instance ? options : []}
        value={query.statistics ?? []}
        onChange={(v) => update({ statistics: v.map((o) => o.value!) })}
        placeholder={instance ? 'Select statistics' : 'Select an instance first'}
        allowCustomValue
        isLoading={!!instance && !options.length && !error}
      />
    </InlineField>
  );
}

/** Pickers for the host_statistic query type. */
export function HostStatisticFields({ query, datasource, update }: Props) {
  const [hosts, setHosts] = useState<Array<SelectableValue<string>>>([]);
  const [error, setError] = useState<string>();
  const instance = firstInstance(query);

  useEffect(() => {
    datasource
      .getHosts(instance)
      .then((h) => {
        setHosts(h);
        setError(undefined);
      })
      .catch((e) => setError(e?.data?.message ?? e?.message ?? 'Could not load hosts'));
  }, [datasource, instance]);

  return (
    <>
      <InlineField
        label="Hosts"
        labelWidth={16}
        grow
        invalid={!!error}
        error={error}
        tooltip="Leave empty to use the hosts of the selected instances"
      >
        <MultiSelect
          inputId="query-editor-hosts"
          options={hosts}
          value={query.hosts ?? []}
          onChange={(v) => update({ hosts: v.map((o) => o.value!) })}
          placeholder="Hosts of the selected instances"
          allowCustomValue
        />
      </InlineField>
      <InlineField label="Metrics" labelWidth={16} grow>
        <MultiSelect
          inputId="query-editor-hostmetrics"
          options={HOST_METRICS}
          value={query.hostMetrics ?? []}
          onChange={(v) => update({ hostMetrics: v.map((o) => o.value!) })}
          placeholder="All four utilisations"
        />
      </InlineField>
    </>
  );
}

/** Pickers for the sql_statistics query type. */
export function SqlStatisticFields({ query, datasource, update }: Props) {
  const [options, setOptions] = useState<Array<SelectableValue<string>>>([]);
  const [error, setError] = useState<string>();
  const instance = firstInstance(query);

  useEffect(() => {
    if (!instance) {
      return;
    }
    datasource
      .getSqlStatistics(instance)
      .then((o) => {
        setOptions(o);
        setError(undefined);
      })
      .catch((e) => setError(e?.data?.message ?? e?.message ?? 'Could not load SQL statistics'));
  }, [datasource, instance]);

  return (
    <>
      <InlineFieldRow>
        <InlineField
          label="Rank by"
          labelWidth={16}
          invalid={!!error}
          error={error}
          tooltip="Offered for the first selected instance's database type"
        >
          <Select
            inputId="query-editor-orderby"
            width={32}
            options={instance ? options : []}
            value={query.orderBy || 'duration'}
            onChange={(v) => update({ orderBy: v.value })}
            allowCustomValue
            isLoading={!!instance && !options.length && !error}
          />
        </InlineField>
        <InlineField label="Top" tooltip="Number of statements to show (at most 50)">
          <Input
            id="query-editor-topn"
            type="number"
            width={8}
            min={1}
            max={50}
            value={query.topN}
            onChange={(e) => update({ topN: Math.max(1, parseInt(e.currentTarget.value, 10) || 1) })}
          />
        </InlineField>
      </InlineFieldRow>
      <InlineField
        label="More columns"
        labelWidth={16}
        grow
        tooltip="Executions, duration and average duration are always shown; at most 10 more"
      >
        <MultiSelect
          inputId="query-editor-sqlcolumns"
          options={instance ? options : []}
          value={query.statistics ?? []}
          onChange={(v) => update({ statistics: v.map((o) => o.value!) })}
          placeholder={instance ? 'Select statistics' : 'Select an instance first'}
          allowCustomValue
        />
      </InlineField>
    </>
  );
}
