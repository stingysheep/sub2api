import { describe, expect, it, vi } from "vitest";
import { reactive } from "vue";

import { useGroupLiveCapability } from "../useGroupLiveCapability";

describe("useGroupLiveCapability", () => {
  it("enables supported live mode and caches the capability result", async () => {
    const getLiveCapability = vi.fn().mockResolvedValue({ supported: true });
    const createForm = reactive({ allow_live: false });
    const editForm = reactive({ allow_live: false });
    const live = useGroupLiveCapability({ createForm, editForm, getLiveCapability });

    await live.toggleLive("create");
    createForm.allow_live = false;
    await live.toggleLive("create");

    expect(createForm.allow_live).toBe(true);
    expect(getLiveCapability).toHaveBeenCalledOnce();
    expect(live.showUnsupportedLiveConfirm.value).toBe(false);
  });

  it("falls back to confirmation when capability lookup fails, without enabling live mode", async () => {
    const createForm = reactive({ allow_live: false });
    const editForm = reactive({ allow_live: false });
    const live = useGroupLiveCapability({
      createForm,
      editForm,
      getLiveCapability: vi.fn().mockRejectedValue(new Error("offline")),
    });

    await live.toggleLive("edit");

    expect(editForm.allow_live).toBe(false);
    expect(live.showUnsupportedLiveConfirm.value).toBe(true);
    live.cancelUnsupportedLive();
    expect(live.showUnsupportedLiveConfirm.value).toBe(false);
    expect(editForm.allow_live).toBe(false);
  });

  it("shares concurrent capability requests and confirms only the pending form", async () => {
    let resolveCapability!: (value: { supported: boolean }) => void;
    const getLiveCapability = vi.fn(
      () => new Promise<{ supported: boolean }>((resolve) => { resolveCapability = resolve; }),
    );
    const createForm = reactive({ allow_live: false });
    const editForm = reactive({ allow_live: false });
    const live = useGroupLiveCapability({ createForm, editForm, getLiveCapability });

    const createToggle = live.toggleLive("create");
    const editToggle = live.toggleLive("edit");
    resolveCapability({ supported: false });
    await Promise.all([createToggle, editToggle]);

    expect(getLiveCapability).toHaveBeenCalledOnce();
    expect(live.pendingLiveForm.value).toBe("edit");
    live.confirmUnsupportedLive();
    expect(createForm.allow_live).toBe(false);
    expect(editForm.allow_live).toBe(true);
  });
});
