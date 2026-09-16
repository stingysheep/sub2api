import { describe, expect, it, vi } from "vitest";

import { useSettingsTabs } from "../useSettingsTabs";

describe("useSettingsTabs", () => {
  it("lazy-mounts a selected tab and keeps earlier tabs mounted", () => {
    const tabs = useSettingsTabs({
      requestAnimationFrame: vi.fn(() => 1),
      getElementById: vi.fn(() => null),
    });

    expect(tabs.isSettingsTabMounted("general")).toBe(true);
    expect(tabs.isSettingsTabMounted("backup")).toBe(false);

    tabs.selectSettingsTab("backup");

    expect(tabs.activeTab.value).toBe("backup");
    expect(tabs.isSettingsTabMounted("general")).toBe(true);
    expect(tabs.isSettingsTabMounted("backup")).toBe(true);
  });

  it("wraps arrow navigation, prevents browser defaults, and focuses the selected tab", () => {
    const focus = vi.fn();
    const requestAnimationFrame = vi.fn((callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    });
    const getElementById = vi.fn(() => ({ focus }) as unknown as HTMLElement);
    const tabs = useSettingsTabs({ requestAnimationFrame, getElementById });
    const event = { key: "ArrowLeft", preventDefault: vi.fn() } as unknown as KeyboardEvent;

    tabs.handleSettingsTabKeydown(event, "general");

    expect(event.preventDefault).toHaveBeenCalledOnce();
    expect(tabs.activeTab.value).toBe("backup");
    expect(tabs.isSettingsTabMounted("backup")).toBe(true);
    expect(getElementById).toHaveBeenCalledWith("settings-tab-backup");
    expect(focus).toHaveBeenCalledOnce();
  });

  it("leaves unrelated keys untouched", () => {
    const tabs = useSettingsTabs({
      requestAnimationFrame: vi.fn(() => 1),
      getElementById: vi.fn(() => null),
    });
    const event = { key: "Enter", preventDefault: vi.fn() } as unknown as KeyboardEvent;

    tabs.handleSettingsTabKeydown(event, "general");

    expect(event.preventDefault).not.toHaveBeenCalled();
    expect(tabs.activeTab.value).toBe("general");
  });
});
