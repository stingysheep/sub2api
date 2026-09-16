import { computed, ref } from "vue";

export type LiveCapability = { supported: boolean; reason?: string };
export type LiveFormTarget = "create" | "edit";
type LiveForm = { allow_live: boolean };

type UseGroupLiveCapabilityOptions = {
  createForm: LiveForm;
  editForm: LiveForm;
  getLiveCapability: () => Promise<LiveCapability>;
};

export function useGroupLiveCapability({
  createForm,
  editForm,
  getLiveCapability,
}: UseGroupLiveCapabilityOptions) {
  const pendingLiveForm = ref<LiveFormTarget | null>(null);
  const showUnsupportedLiveConfirm = computed(() => pendingLiveForm.value !== null);
  const liveCapability = ref<LiveCapability | null>(null);
  let liveCapabilityRequest: Promise<LiveCapability> | null = null;

  const formFor = (target: LiveFormTarget): LiveForm =>
    target === "create" ? createForm : editForm;

  const loadLiveCapability = async (): Promise<LiveCapability> => {
    if (liveCapability.value) return liveCapability.value;
    if (!liveCapabilityRequest) {
      liveCapabilityRequest = getLiveCapability()
        .catch(() => ({ supported: false }))
        .finally(() => {
          liveCapabilityRequest = null;
        });
    }
    liveCapability.value = await liveCapabilityRequest;
    return liveCapability.value ?? { supported: false };
  };

  const toggleLive = async (target: LiveFormTarget): Promise<void> => {
    const form = formFor(target);
    if (form.allow_live) {
      form.allow_live = false;
      return;
    }
    const capability = await loadLiveCapability();
    if (capability.supported) {
      form.allow_live = true;
      return;
    }
    pendingLiveForm.value = target;
  };

  const confirmUnsupportedLive = (): void => {
    if (pendingLiveForm.value) formFor(pendingLiveForm.value).allow_live = true;
    pendingLiveForm.value = null;
  };

  const cancelUnsupportedLive = (): void => {
    pendingLiveForm.value = null;
  };

  return {
    pendingLiveForm,
    showUnsupportedLiveConfirm,
    liveCapability,
    loadLiveCapability,
    toggleLive,
    confirmUnsupportedLive,
    cancelUnsupportedLive,
  };
}
