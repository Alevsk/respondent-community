import { createContext, useContext } from 'react';
export type DetailVariant = 'clean' | 'classic';
export const DetailVariantContext = createContext<DetailVariant>('classic');
export function useDetailVariant(): DetailVariant {
  return useContext(DetailVariantContext);
}
