import { Suspense } from "react";
import { RootScreen } from "@/components/RootScreen";

export default function Page() {
  return (
    <Suspense>
      <RootScreen />
    </Suspense>
  );
}
