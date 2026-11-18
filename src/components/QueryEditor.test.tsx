import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { DataSource } from '../datasource';
import { QueryEditor } from './QueryEditor';
import { DBmarlinQuery, DEFAULT_QUERY } from '../types';

jest.mock('@grafana/runtime', () => ({
  ...jest.requireActual('@grafana/runtime'),
  getTemplateSrv: () => ({ getVariables: () => [{ name: 'instance' }] }),
}));

function setup(query: Partial<DBmarlinQuery> = {}, instances: unknown = Promise.resolve([{ label: 'repo', value: '3' }])) {
  const onChange = jest.fn();
  const onRunQuery = jest.fn();
  const datasource = {
    getInstances: jest.fn().mockReturnValue(instances),
    getStatistics: jest.fn().mockResolvedValue([{ label: 'blks_hit', value: 'blks_hit' }]),
    getSqlStatistics: jest.fn().mockResolvedValue([{ label: 'duration', value: 'duration' }]),
    getHosts: jest.fn().mockResolvedValue([{ label: 'docker-linux', value: '7' }]),
  } as unknown as DataSource;
  render(
    <QueryEditor
      query={{ refId: 'A', ...DEFAULT_QUERY, ...query } as DBmarlinQuery}
      onChange={onChange}
      onRunQuery={onRunQuery}
      datasource={datasource}
    />
  );
  return { onChange, onRunQuery, datasource };
}

describe('QueryEditor', () => {
  it('loads the instances', async () => {
    const { datasource } = setup();
    expect(await screen.findByText('Instances')).toBeInTheDocument();
    expect(datasource.getInstances).toHaveBeenCalledTimes(1);
  });

  it('hides top N and other until grouped', async () => {
    const { datasource } = setup();
    await waitFor(() => expect(datasource.getInstances).toHaveBeenCalled());
    expect(screen.queryByLabelText('Top')).not.toBeInTheDocument();
    expect(screen.queryByText('Show other')).not.toBeInTheDocument();
  });

  it('shows top N and other when grouped, and runs the query on change', async () => {
    const { onChange, onRunQuery } = setup({ groupBy: 'waitevent' });
    const top = await screen.findByLabelText('Top');
    fireEvent.change(top, { target: { value: '5' } });
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ groupBy: 'waitevent', topN: 5 }));
    expect(onRunQuery).toHaveBeenCalled();
  });

  it('shows the time series options only for time series', async () => {
    const { datasource } = setup();
    await waitFor(() => expect(datasource.getInstances).toHaveBeenCalled());
    expect(screen.getByText('Metric')).toBeInTheDocument();
    expect(screen.getByText('Combine instances')).toBeInTheDocument();
  });

  it('lists a dimension, without the time series options, for a table', async () => {
    setup({ queryType: 'activity_table', groupBy: 'waitevent' });
    expect(await screen.findByLabelText('Top')).toBeInTheDocument();
    expect(screen.getByText('List')).toBeInTheDocument();
    expect(screen.queryByText('Metric')).not.toBeInTheDocument();
    expect(screen.queryByText('Combine instances')).not.toBeInTheDocument();
    expect(screen.queryByText('Show other')).not.toBeInTheDocument();
  });

  it('offers the statistics of the first instance for database statistics', async () => {
    const { datasource } = setup({ queryType: 'db_statistic', instances: ['3'] });
    expect(await screen.findByText('Statistics')).toBeInTheDocument();
    expect(datasource.getStatistics).toHaveBeenCalledWith('3');
    expect(screen.queryByText('Metric')).not.toBeInTheDocument();
    expect(screen.queryByText('Group by')).not.toBeInTheDocument();
  });

  it('does not ask for statistics while the instance is a dashboard variable', async () => {
    const { datasource } = setup({ queryType: 'db_statistic', instances: ['$instance'] });
    await waitFor(() => expect(datasource.getInstances).toHaveBeenCalled());
    expect(datasource.getStatistics).not.toHaveBeenCalled();
  });

  it('loads the instance hosts for host metrics', async () => {
    const { datasource } = setup({ queryType: 'host_statistic', instances: ['3'] });
    expect(await screen.findByText('Hosts')).toBeInTheDocument();
    expect(screen.getByText('Metrics')).toBeInTheDocument();
    expect(datasource.getHosts).toHaveBeenCalledWith('3');
  });

  it('offers SQL statistics to rank by, with a top N, and no activity options', async () => {
    const { datasource, onChange } = setup({ queryType: 'sql_statistics', instances: ['3'] });
    expect(await screen.findByText('Rank by')).toBeInTheDocument();
    expect(datasource.getSqlStatistics).toHaveBeenCalledWith('3');
    expect(screen.queryByText('Group by')).not.toBeInTheDocument();
    expect(screen.queryByText('Metric')).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Top'), { target: { value: '7' } });
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ queryType: 'sql_statistics', topN: 7 }));
  });

  it('offers the instances and the dashboard variables', async () => {
    setup();
    fireEvent.keyDown(screen.getByLabelText('Instances'), { key: 'ArrowDown' });
    expect(await screen.findByText('repo')).toBeInTheDocument();
    expect(screen.getByText('$instance')).toBeInTheDocument();
  });

  it('shows why the instances could not be loaded', async () => {
    setup({}, Promise.reject({ data: { message: 'Unauthorized' } }));
    expect(await screen.findByText('Unauthorized')).toBeInTheDocument();
  });

  it('cannot leave a table ungrouped', async () => {
    const { onChange, onRunQuery } = setup({ queryType: 'activity_timeseries', groupBy: 'none' });
    fireEvent.keyDown(screen.getByLabelText('Query type'), { key: 'ArrowDown' });
    fireEvent.click(await screen.findByText('Activity top N table'));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ queryType: 'activity_table', groupBy: 'waitevent' })
    );
    expect(onRunQuery).toHaveBeenCalled();
  });

  it('keeps top N at least 1', async () => {
    const { onChange } = setup({ groupBy: 'waitevent' });
    fireEvent.change(await screen.findByLabelText('Top'), { target: { value: '0' } });
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ topN: 1 }));
  });
});
