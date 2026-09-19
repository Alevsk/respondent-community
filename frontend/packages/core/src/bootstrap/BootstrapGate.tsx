import React from 'react';
import type { BootstrapState } from './types';
import AppLoadingOverlay from './AppLoadingOverlay';

export interface BootstrapGateProps extends BootstrapState {
  children: React.ReactNode;
}

/**
 * Presentational bootstrap gate. Shows the loading overlay until the app
 * signals readiness via the BootstrapState props.
 *
 * In the app, wrap this with the `useAppBootstrap` hook to wire up state.
 */
const BootstrapGate: React.FC<BootstrapGateProps> = ({
  ready,
  error,
  currentLabel,
  retry,
  children,
}) => {
  if (!ready) {
    return <AppLoadingOverlay label={currentLabel} error={error} onRetry={retry} />;
  }

  return <>{children}</>;
};

export default BootstrapGate;
