import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { DataSourceSettings } from '@grafana/data';
import { ConfigEditor } from './ConfigEditor';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

function setup(overrides: Partial<DataSourceSettings<MyDataSourceOptions, MySecureJsonData>> = {}) {
  const onOptionsChange = jest.fn();
  const options = {
    jsonData: {},
    secureJsonFields: {},
    secureJsonData: {},
    ...overrides,
  } as DataSourceSettings<MyDataSourceOptions, MySecureJsonData>;
  render(<ConfigEditor options={options} onOptionsChange={onOptionsChange} />);
  return { onOptionsChange, options };
}

describe('ConfigEditor', () => {
  it('stores the URL in jsonData', () => {
    const { onOptionsChange } = setup();
    fireEvent.change(screen.getByPlaceholderText('https://dbmarlin.example.com:9090'), {
      target: { value: 'https://db:9090' },
    });
    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({ jsonData: expect.objectContaining({ url: 'https://db:9090' }) })
    );
  });

  it('keeps the password out of jsonData', () => {
    const { onOptionsChange } = setup();
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret' } });
    const saved = onOptionsChange.mock.calls[0][0];
    expect(saved.secureJsonData).toEqual({ password: 'secret' });
    expect(JSON.stringify(saved.jsonData)).not.toContain('secret');
  });

  it('shows a saved password as configured and resets it', () => {
    const { onOptionsChange } = setup({ secureJsonFields: { password: true } });
    expect(screen.getByDisplayValue('configured')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /reset/i }));
    expect(onOptionsChange).toHaveBeenCalledWith(expect.objectContaining({ secureJsonFields: { password: false } }));
  });
});
