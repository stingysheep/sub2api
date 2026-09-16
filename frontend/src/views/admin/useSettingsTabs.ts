import { reactive, ref } from "vue";

export type SettingsTab =
  | "general"
  | "agreement"
  | "features"
  | "security"
  | "users"
  | "gateway"
  | "payment"
  | "email"
  | "backup";

export const settingsTabs = [
  { key: "general" as SettingsTab, icon: "home" as const },
  { key: "agreement" as SettingsTab, icon: "document" as const },
  { key: "features" as SettingsTab, icon: "bolt" as const },
  { key: "security" as SettingsTab, icon: "shield" as const },
  { key: "users" as SettingsTab, icon: "user" as const },
  { key: "gateway" as SettingsTab, icon: "server" as const },
  { key: "payment" as SettingsTab, icon: "creditCard" as const },
  { key: "email" as SettingsTab, icon: "mail" as const },
  { key: "backup" as SettingsTab, icon: "database" as const },
];

const keyboardActions = {
  ArrowLeft: -1,
  ArrowUp: -1,
  ArrowRight: 1,
  ArrowDown: 1,
  Home: "first",
  End: "last",
} as const;

type SettingsTabsDependencies = {
  requestAnimationFrame?: (callback: FrameRequestCallback) => number;
  getElementById?: (id: string) => HTMLElement | null;
};

export function useSettingsTabs(deps: SettingsTabsDependencies = {}) {
  const activeTab = ref<SettingsTab>("general");
  const mountedSettingsTabs = reactive<Set<SettingsTab>>(new Set(["general"]));
  const requestFrame = deps.requestAnimationFrame ?? window.requestAnimationFrame.bind(window);
  const getTabElement = deps.getElementById ?? document.getElementById.bind(document);

  function isSettingsTabMounted(tab: SettingsTab): boolean {
    return mountedSettingsTabs.has(tab);
  }

  function selectSettingsTab(tab: SettingsTab): void {
    mountedSettingsTabs.add(tab);
    activeTab.value = tab;
  }

  function focusSettingsTab(tab: SettingsTab): void {
    requestFrame(() => getTabElement(`settings-tab-${tab}`)?.focus());
  }

  function handleSettingsTabKeydown(event: KeyboardEvent, tab: SettingsTab): void {
    const action = keyboardActions[event.key as keyof typeof keyboardActions];
    if (action === undefined) return;

    event.preventDefault();
    const currentIndex = settingsTabs.findIndex((item) => item.key === tab);
    let nextIndex = currentIndex < 0 ? 0 : currentIndex;
    if (action === "first") nextIndex = 0;
    else if (action === "last") nextIndex = settingsTabs.length - 1;
    else nextIndex = (nextIndex + action + settingsTabs.length) % settingsTabs.length;

    const nextTab = settingsTabs[nextIndex]?.key;
    if (!nextTab) return;
    selectSettingsTab(nextTab);
    focusSettingsTab(nextTab);
  }

  return {
    activeTab,
    settingsTabs,
    isSettingsTabMounted,
    selectSettingsTab,
    handleSettingsTabKeydown,
  };
}
