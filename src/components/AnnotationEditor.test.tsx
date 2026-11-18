import React from 'react';
import { render, screen } from '@testing-library/react';
import { DataSource } from '../datasource';
import { AnnotationEditor } from './AnnotationEditor';
import { DBmarlinQuery } from '../types';

describe('AnnotationEditor', () => {
  it('asks for instances and the kinds of event', async () => {
    const datasource = {
      getInstances: jest.fn().mockResolvedValue([{ label: 'repo', value: '3' }]),
    } as unknown as DataSource;
    render(
      <AnnotationEditor
        query={{ refId: 'Anno', queryType: 'events', instances: ['3'] } as DBmarlinQuery}
        onChange={jest.fn()}
        onRunQuery={jest.fn()}
        datasource={datasource}
      />
    );
    expect(await screen.findByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('Show')).toBeInTheDocument();
    expect(datasource.getInstances).toHaveBeenCalledTimes(1);
  });
});
