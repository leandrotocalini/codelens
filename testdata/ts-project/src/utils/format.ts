import { Config } from '../types';

export type FormatOptions = {
  locale: string;
  currency: string;
};

export function formatCurrency(amount: number, options: FormatOptions): string {
  return `${options.currency}${amount}`;
}

export function formatDate(date: Date): string {
  return date.toISOString();
}

export class Formatter {
  constructor(private config: Config) {}

  format(value: string): string {
    return value;
  }
}
