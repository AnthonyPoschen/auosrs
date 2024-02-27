# below link explains the functions
# https://stackoverflow.com/a/10858332
check_defined = \
    $(strip $(foreach 1,$1, \
        $(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
    $(if $(value $1),, \
        $(error Undefined $1$(if $2, ($2))$(if $(value @), \
		required by target `$@`)))

gen_wiseoldman_secret:
	@:$(call check_defined, TOKEN, wiseoldman token)
	@:$(call check_defined, ENV, environment for secret)
	kubectl create secret generic client-info-secret \
		--namespace app-auosrs \
		--dry-run=client \
		--from-literal=token="${token}" \
		--output="yaml" | kubeseal -o yaml > ./${ENV}/wiseoldman.yaml
