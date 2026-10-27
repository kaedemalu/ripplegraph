"use client";

import { createContext, useContext } from "react";

/** カスタムノードから呼ぶ操作。ノードの data を安定させるため context で渡す。 */
export type GraphActions = {
  toggleModule: (id: string) => void;
};

export const GraphActionsContext = createContext<GraphActions>({ toggleModule: () => {} });

export const useGraphActions = () => useContext(GraphActionsContext);
