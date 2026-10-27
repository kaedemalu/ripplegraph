/// <reference lib="webworker" />
// elkjs のレイアウト計算を行う Web Worker。
//
// elkjs の worker スクリプトは、worker のグローバルスコープで読み込まれると self.onmessage に
// 自身のディスパッチャを登録する。メインスレッドの elk-api（layoutClient.ts）がこの worker に
// グラフを送り、レイアウト計算（重い処理）はすべてこの worker 上で行われる。
import "elkjs/lib/elk-worker.min.js";
