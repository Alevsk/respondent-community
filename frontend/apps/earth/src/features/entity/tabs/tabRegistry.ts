/**
 * Tab Registry — plugin-based tab system for the Entity Detail Panel.
 *
 * New entity tabs can be added here without modifying the panel component.
 * Tabs can be filtered by layer type and ordered by priority.
 */

import React from 'react';
import type { ReactNode } from 'react';
import type { EntityDetailResponse } from '../../../shared/api/queries';

export interface EntityTabProps {
  entityId: string;
  layerType: string;
  detail: EntityDetailResponse | undefined;
  isLoading: boolean;
}

export interface EntityTab {
  id: string;
  label: string;
  icon?: ReactNode;
  /** Only show this tab for certain layer types. Omit = show for all. */
  layerTypes?: string[];
  /** Priority for tab ordering (lower = leftmost) */
  priority: number;
  /** The tab content component */
  component: React.ComponentType<EntityTabProps>;
}

const TAB_REGISTRY: EntityTab[] = [];

export function registerTab(tab: EntityTab): void {
  TAB_REGISTRY.push(tab);
}

export function getTabsForLayerType(layerType: string): EntityTab[] {
  return TAB_REGISTRY.filter((tab) => !tab.layerTypes || tab.layerTypes.includes(layerType)).sort(
    (a, b) => a.priority - b.priority,
  );
}
