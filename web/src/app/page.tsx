import { Suspense } from "react";
import { MatrixScreen } from "@/components/MatrixScreen";

export default function Page() {
  return (
    <Suspense>
      <MatrixScreen />
    </Suspense>
  );
}
