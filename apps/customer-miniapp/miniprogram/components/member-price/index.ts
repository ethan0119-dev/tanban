Component({
  properties: {
    memberPrice: { type: Number, value: 0 },
    originalPrice: { type: Number, value: 0 },
    label: { type: String, value: "会员价" },
    variant: { type: String, value: "inline" },
    selected: { type: Boolean, value: false },
    showOriginal: { type: Boolean, value: true },
  },
});
