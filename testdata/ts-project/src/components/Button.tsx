import React from 'react';
import { ButtonProps } from '../types';

export interface ButtonOptions {
  variant: 'primary' | 'secondary';
  size: 'sm' | 'md' | 'lg';
}

export function Button({ label, onClick }: ButtonProps) {
  return <button onClick={onClick}>{label}</button>;
}

export const IconButton = ({ icon }: { icon: string }) => {
  return <button>{icon}</button>;
};

const internalHelper = () => {};
