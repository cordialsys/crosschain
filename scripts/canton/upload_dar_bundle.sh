find /tmp/canton-network-utility-dars-0.12.0 -name '*.dar' | while read -r dar; do
  echo "Uploading $dar"
  bytes="$(base64 -i "$dar" | tr -d '\n')"
  jq -n --arg bytes "$bytes" --arg desc "$(basename "$dar")" '{
    dars: [{bytes: $bytes, description: $desc}],
    vet_all_packages: true,
    synchronize_vetting: true
  }' | grpcurl \
    -H "Authorization: Bearer '"$AUTH_TOKEN"'" \
    -d @ \
    "$PARTICIPANT_ADMIN_HOST" \
    com.digitalasset.canton.admin.participant.v30.PackageService.UploadDar
done

