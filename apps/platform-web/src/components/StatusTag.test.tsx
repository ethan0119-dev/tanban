import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StatusTag } from './StatusTag';

describe('StatusTag', () => {
  it('展示中文业务状态', () => {
    render(<StatusTag status="active" />);
    expect(screen.getByText('正常')).toBeInTheDocument();
  });
});
