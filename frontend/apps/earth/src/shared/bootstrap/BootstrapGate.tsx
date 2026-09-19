// App-specific BootstrapGate — wires useAppBootstrap hook to the core BootstrapGate component.
// The core BootstrapGate is a presentational component that accepts BootstrapState as props.
import React from 'react';
import { BootstrapGate as CoreBootstrapGate } from '@respondent/core';
import { useAppBootstrap } from './useAppBootstrap';

const BootstrapGate: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const bootstrapState = useAppBootstrap();

  return <CoreBootstrapGate {...bootstrapState}>{children}</CoreBootstrapGate>;
};

export default BootstrapGate;
